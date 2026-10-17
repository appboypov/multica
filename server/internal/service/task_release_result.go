package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

var ErrReleaseResultEmpty = errors.New("release coordination ended without a nonempty result or explicit release report")

const oversizedReleaseReportNotice = "This coordination run completed, but its output was too large to post safely. The raw output was not posted."

// The task status CAS and this write share one transaction. A replay cannot
// insert another report, and deleting the release cannot recreate its subject.
func writeReleaseCompletionReport(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, result []byte) (*db.Comment, error) {
	rel, err := q.GetReleaseForTask(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	_, err = q.GetReleaseReportBySourceTask(ctx, db.GetReleaseReportBySourceTaskParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, SourceTaskID: task.ID})
	if err == nil {
		return nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("read release source-task report: %w", err)
	}
	var payload protocol.TaskCompletedPayload
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, fmt.Errorf("decode release final result: %w", err)
	}
	body := redact.Text(payload.Output)
	if strings.TrimSpace(body) == "" {
		return nil, ErrReleaseResultEmpty
	}
	// Same bound as an issue run's synthesized fallback comment.
	if utf8.RuneCountInString(body) > maxSynthesizedFallbackCommentRunes {
		body = oversizedReleaseReportNotice
	}
	var parentID pgtype.UUID
	if task.TriggerCommentID.Valid && len(task.DeliveredCommentIds) <= 1 {
		parent, err := q.GetCommentInWorkspace(ctx, db.GetCommentInWorkspaceParams{ID: task.TriggerCommentID, WorkspaceID: rel.WorkspaceID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil && parent.ReleaseID == rel.ID {
			parentID = parent.ID
		}
	}
	report, err := q.CreateReleaseComment(ctx, db.CreateReleaseCommentParams{ID: dbid.NewV7(), WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, AuthorType: "agent", AuthorID: task.AgentID, Content: body, Type: "comment", ParentID: parentID, SourceTaskID: task.ID})
	if err != nil {
		return nil, fmt.Errorf("write release final result: %w", err)
	}
	return &report, nil
}
