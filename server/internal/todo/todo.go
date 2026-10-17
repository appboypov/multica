// Package todo owns the transitions that ask a member to act.
package todo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

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

type Service struct {
	Queries *db.Queries
	Bus     *events.Bus
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
	row, err := s.Queries.OpenTodo(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s todo: %w", p.Kind, err)
	}
	return s.PublishOpened(ctx, row)
}

// PublishOpened runs after a caller's transaction commits.
func (s Service) PublishOpened(ctx context.Context, row db.Todo) error {
	return s.publishChanges(ctx, []pgtype.UUID{row.ID}, true)
}

func (s Service) Close(ctx context.Context, p db.CloseTodosParams) error {
	rows, err := s.Queries.CloseTodos(ctx, p)
	if err != nil {
		return fmt.Errorf("close %s todos: %w", p.Kind, err)
	}
	return s.PublishClosed(ctx, rows)
}

func (s Service) CloseOnIssue(ctx context.Context, p db.CloseTodosOnIssueParams) error {
	rows, err := s.Queries.CloseTodosOnIssue(ctx, p)
	if err != nil {
		return fmt.Errorf("close issue todos: %w", err)
	}
	return s.PublishClosed(ctx, rows)
}

func (s Service) CloseInWorkspace(ctx context.Context, p db.CloseTodosInWorkspaceParams) error {
	rows, err := s.Queries.CloseTodosInWorkspace(ctx, p)
	if err != nil {
		return fmt.Errorf("close workspace todos: %w", err)
	}
	return s.PublishClosed(ctx, rows)
}

// MoveFromDeletedIssue publishes moved rows as opened: their deleted issue hid them.
func (s Service) MoveFromDeletedIssue(ctx context.Context, p db.MoveTodosFromDeletedIssueParams) error {
	rows, err := s.Queries.MoveTodosFromDeletedIssue(ctx, p)
	if err != nil {
		return fmt.Errorf("move %s todos: %w", p.Kind, err)
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return s.publishChanges(ctx, ids, true)
}

// PublishClosed also serves member-driven completion after its database write.
func (s Service) PublishClosed(ctx context.Context, rows []db.Todo) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return s.publishChanges(ctx, ids, false)
}

func (s Service) publishChanges(ctx context.Context, ids []pgtype.UUID, opening bool) error {
	changes, err := s.Queries.ListTodoVisibilityChanges(ctx, db.ListTodoVisibilityChangesParams{Ids: ids, Opening: opening})
	if err != nil {
		return fmt.Errorf("query todo visibility changes: %w", err)
	}
	for _, row := range changes {
		event := events.Event{Type: protocol.EventTodoClosed, WorkspaceID: util.UUIDToString(row.WorkspaceID)}
		payload := map[string]any{"member_id": util.UUIDToString(row.MemberID)}
		if row.IsVisible {
			projected, err := s.Queries.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: row.ID, WorkspaceID: row.WorkspaceID, MemberID: row.MemberID})
			if err != nil {
				return fmt.Errorf("project opened todo: %w", err)
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
		if s.Bus != nil {
			s.Bus.Publish(event)
		}
		slog.Debug("todo visibility changed", "todo_id", util.UUIDToString(row.ID), "kind", row.Kind, "visible", row.IsVisible)
	}
	return nil
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
