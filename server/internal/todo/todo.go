// Package todo owns the transitions that ask a member to act.
package todo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	Question          = "question"
	Mention           = "mention"
	Assigned          = "assigned"
	AgentBlocked      = "agent_blocked"
	RunFailed         = "run_failed"
	AutopilotPaused   = "autopilot_paused"
	QuotaReached      = "quota_reached"
	QuickCreateFailed = "quick_create_failed"
	Review            = "review"
)

// TxStarter abstracts transaction creation (satisfied by pgxpool.Pool).
type TxStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service writes to-dos and publishes their visible-list transitions.
// Question and review writers set TxStarter: concurrent writes that decide one
// review then see each other, so the review hides and reveals exactly once.
type Service struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Bus       *events.Bus
}

// Open publishes only a newly inserted row. Duplicate source events are inert.
func (s Service) Open(ctx context.Context, p db.OpenTodoParams) error {
	if !p.ID.Valid {
		p.ID = dbid.NewV7()
	}
	if len(p.Details) == 0 {
		p.Details = []byte(`{}`)
	}
	// A to-do names a member or an agent as its actor. A system event opens it without one.
	if p.ActorType.String != "member" && p.ActorType.String != "agent" {
		p.ActorType, p.ActorID = pgtype.Text{}, pgtype.UUID{}
	}
	_, err := s.write(ctx, true, func(q *db.Queries) ([]db.Todo, error) {
		row, err := q.OpenTodo(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("open %s todo: %w", p.Kind, err)
		}
		return []db.Todo{row}, nil
	})
	return err
}

// PublishOpened runs after a caller's transaction commits, for rows whose
// visibility no other row decides.
func (s Service) PublishOpened(ctx context.Context, row db.Todo) error {
	changes, err := visibilityChanges(ctx, s.Queries, []db.Todo{row}, true)
	if err != nil {
		return err
	}
	s.publish(changes)
	return nil
}

func (s Service) Close(ctx context.Context, p db.CloseTodosParams) error {
	_, err := s.write(ctx, false, func(q *db.Queries) ([]db.Todo, error) {
		rows, err := q.CloseTodos(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("close %s todos: %w", p.Kind, err)
		}
		return rows, nil
	})
	return err
}

func (s Service) CloseOnIssue(ctx context.Context, p db.CloseTodosOnIssueParams) error {
	_, err := s.write(ctx, false, func(q *db.Queries) ([]db.Todo, error) {
		rows, err := q.CloseTodosOnIssue(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("close issue todos: %w", err)
		}
		return rows, nil
	})
	return err
}

func (s Service) CloseInWorkspace(ctx context.Context, p db.CloseTodosInWorkspaceParams) error {
	_, err := s.write(ctx, false, func(q *db.Queries) ([]db.Todo, error) {
		rows, err := q.CloseTodosInWorkspace(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("close workspace todos: %w", err)
		}
		return rows, nil
	})
	return err
}

// MoveFromDeletedIssue publishes moved rows as opened: their deleted issue hid them.
func (s Service) MoveFromDeletedIssue(ctx context.Context, p db.MoveTodosFromDeletedIssueParams) error {
	_, err := s.write(ctx, true, func(q *db.Queries) ([]db.Todo, error) {
		rows, err := q.MoveTodosFromDeletedIssue(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("move %s todos: %w", p.Kind, err)
		}
		return rows, nil
	})
	return err
}

// MarkDone closes the member's own open to-do. pgx.ErrNoRows means it was
// already closed or is not theirs.
func (s Service) MarkDone(ctx context.Context, p db.MarkTodoDoneParams) (db.Todo, error) {
	rows, err := s.write(ctx, false, func(q *db.Queries) ([]db.Todo, error) {
		row, err := q.MarkTodoDone(ctx, p)
		if err != nil {
			return nil, err
		}
		return []db.Todo{row}, nil
	})
	if err != nil {
		return db.Todo{}, err
	}
	return rows[0], nil
}

// write runs a to-do write and its visibility query in one transaction and
// publishes the transitions after commit.
func (s Service) write(ctx context.Context, opening bool, apply func(*db.Queries) ([]db.Todo, error)) ([]db.Todo, error) {
	q := s.Queries
	var tx pgx.Tx
	if s.TxStarter != nil {
		var err error
		tx, err = s.TxStarter.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin todo write: %w", err)
		}
		defer tx.Rollback(ctx)
		q = q.WithTx(tx)
	}
	rows, err := apply(q)
	if err != nil {
		return nil, err
	}
	changes, err := visibilityChanges(ctx, q, rows, opening)
	if err != nil {
		return nil, err
	}
	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit todo write: %w", err)
		}
	}
	s.publish(changes)
	return rows, nil
}

// visibilityChanges builds the events for rows the current transaction wrote.
// It first locks each member's review on each issue the rows touch, so a
// concurrent question or review write waits and then sees this write.
func visibilityChanges(ctx context.Context, q *db.Queries, rows []db.Todo, opening bool) ([]events.Event, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	locks := map[string]db.LockTodoReviewVisibilityParams{}
	ids := make([]pgtype.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
		if row.IssueID.Valid && (row.Kind == Question || row.Kind == Review) {
			key := util.UUIDToString(row.WorkspaceID) + util.UUIDToString(row.MemberID) + util.UUIDToString(row.IssueID)
			locks[key] = db.LockTodoReviewVisibilityParams{WorkspaceID: row.WorkspaceID, MemberID: row.MemberID, IssueID: row.IssueID}
		}
	}
	// A fixed lock order keeps two multi-member writes from deadlocking.
	for _, key := range slices.Sorted(maps.Keys(locks)) {
		if err := q.LockTodoReviewVisibility(ctx, locks[key]); err != nil {
			return nil, fmt.Errorf("lock todo review visibility: %w", err)
		}
	}
	changes, err := q.ListTodoVisibilityChanges(ctx, db.ListTodoVisibilityChangesParams{Ids: ids, Opening: opening})
	if err != nil {
		return nil, fmt.Errorf("query todo visibility changes: %w", err)
	}
	notices := make([]events.Event, 0, len(changes))
	for _, row := range changes {
		event := events.Event{Type: protocol.EventTodoClosed, WorkspaceID: util.UUIDToString(row.WorkspaceID)}
		payload := map[string]any{"member_id": util.UUIDToString(row.MemberID)}
		if row.IsVisible {
			projected, err := q.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: row.ID, WorkspaceID: row.WorkspaceID, MemberID: row.MemberID})
			if err != nil {
				return nil, fmt.Errorf("project opened todo: %w", err)
			}
			event.Type = protocol.EventTodoOpened
			payload["todo"] = Response(projected)
		} else {
			payload["todo_id"] = util.UUIDToString(row.ID)
			payload["kind"] = row.Kind
			payload["done_by"] = nil
			if row.DoneBy.Valid {
				payload["done_by"] = row.DoneBy.String
			}
		}
		event.Payload = payload
		notices = append(notices, event)
		slog.Debug("todo visibility changed", "todo_id", util.UUIDToString(row.ID), "kind", row.Kind, "visible", row.IsVisible)
	}
	return notices, nil
}

func (s Service) publish(notices []events.Event) {
	if s.Bus == nil {
		return
	}
	for _, event := range notices {
		s.Bus.Publish(event)
	}
}

// Response is shared by the API and opened events, including live source fields.
func Response(row db.GetTodoForMemberRow) map[string]any {
	text := func(v pgtype.Text) any {
		if v.Valid {
			return v.String
		}
		return nil
	}
	uuid := func(v pgtype.UUID) any {
		if v.Valid {
			return util.UUIDToString(v)
		}
		return nil
	}
	var due, done any
	if row.DueDate.Valid {
		due = row.DueDate.Time.Format("2006-01-02")
	}
	if row.DoneAt.Valid {
		done = row.DoneAt.Time
	}
	return map[string]any{
		"id": util.UUIDToString(row.ID), "workspace_id": util.UUIDToString(row.WorkspaceID), "kind": row.Kind,
		"issue_id": uuid(row.IssueID), "issue_identifier": text(row.IssueIdentifier), "issue_title": text(row.IssueTitle), "issue_status": text(row.IssueStatus),
		"question_id": uuid(row.QuestionID), "question_answer_type": text(row.QuestionAnswerType), "comment_id": uuid(row.CommentID), "task_id": uuid(row.TaskID), "autopilot_id": uuid(row.AutopilotID),
		"actor_type": text(row.ActorType), "actor_id": uuid(row.ActorID), "title": row.Title, "body": text(row.Body), "details": json.RawMessage(row.Details),
		"due_date": due, "overdue": row.Overdue, "created_at": row.CreatedAt.Time, "done_at": done, "done_by": text(row.DoneBy),
	}
}
