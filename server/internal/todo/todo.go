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
	row, err := s.Queries.OpenTodo(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s todo: %w", p.Kind, err)
	}
	projected, err := s.Queries.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: row.ID, WorkspaceID: row.WorkspaceID, MemberID: row.MemberID})
	if err != nil {
		return fmt.Errorf("project opened todo: %w", err)
	}
	if s.Bus != nil {
		s.Bus.Publish(events.Event{Type: protocol.EventTodoOpened, WorkspaceID: util.UUIDToString(row.WorkspaceID), Payload: map[string]any{"member_id": util.UUIDToString(row.MemberID), "todo": Response(projected)}})
	}
	slog.Debug("todo opened", "todo_id", util.UUIDToString(row.ID), "kind", row.Kind)
	return nil
}

func (s Service) Close(ctx context.Context, p db.CloseTodosParams) error {
	rows, err := s.Queries.CloseTodos(ctx, p)
	if err != nil {
		return fmt.Errorf("close %s todos: %w", p.Kind, err)
	}
	s.PublishClosed(rows)
	return nil
}

func (s Service) CloseOnIssue(ctx context.Context, p db.CloseTodosOnIssueParams) error {
	rows, err := s.Queries.CloseTodosOnIssue(ctx, p)
	if err != nil {
		return fmt.Errorf("close issue todos: %w", err)
	}
	s.PublishClosed(rows)
	return nil
}

// PublishClosed also serves member-driven completion after its database write.
func (s Service) PublishClosed(rows []db.Todo) {
	for _, row := range rows {
		if s.Bus != nil {
			s.Bus.Publish(events.Event{Type: protocol.EventTodoClosed, WorkspaceID: util.UUIDToString(row.WorkspaceID), Payload: map[string]any{"member_id": util.UUIDToString(row.MemberID), "todo_id": util.UUIDToString(row.ID), "kind": row.Kind, "done_by": row.DoneBy.String}})
		}
		slog.Debug("todo closed", "todo_id", util.UUIDToString(row.ID), "kind", row.Kind, "done_by", row.DoneBy.String)
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
