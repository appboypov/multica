package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) hydrateReleaseClaim(ctx context.Context, task *db.AgentTaskQueue, runtime db.AgentRuntime, resp *AgentTaskResponse) ([]CoalescedCommentData, error) {
	rel, err := h.Queries.GetReleaseForTask(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if rel.WorkspaceID != runtime.WorkspaceID {
		return nil, pgx.ErrNoRows
	}
	agent, squadID, err := service.ResolveReleaseTaskOwner(ctx, h.Queries, rel)
	if err != nil {
		return nil, err
	}
	if agent.ID != task.AgentID || agent.RuntimeID != task.RuntimeID || squadID != task.SquadID {
		return nil, pgx.ErrNoRows
	}
	repo, err := h.Queries.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID})
	if err != nil {
		return nil, err
	}
	issues, err := h.releaseIssues(ctx, h.Queries, rel)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(issues)
	if err != nil {
		return nil, err
	}
	snapshot := &protocol.ClaimRelease{ID: uuidToString(rel.ID), RepositoryID: uuidToString(repo.ID), RepositoryName: repo.FullName, ProjectID: uuidToString(repo.ProjectID), Version: rel.Version, Title: rel.Title, Description: rel.Description, Issues: encoded, AllSuccessful: len(issues) > 0}
	for _, issue := range issues {
		if issue.StatusCategory != "done" {
			snapshot.AllSuccessful = false
		}
		if issue.StatusCategory != "done" && issue.StatusCategory != "closed" {
			snapshot.RemainingOpen++
		}
	}
	resp.ReleaseID, resp.Release, resp.WorkspaceID = snapshot.ID, snapshot, uuidToString(rel.WorkspaceID)
	resp.ThreadName = fmt.Sprintf("v%s %s", rel.Version, rel.Title)
	resp.QuestionsEnabled = false
	resp.IsLeaderTask = squadID.Valid
	resp.SquadID = uuidToString(squadID)
	if squadID.Valid {
		squad, err := h.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: squadID, WorkspaceID: rel.WorkspaceID})
		if err != nil {
			return nil, err
		}
		if squad.LeaderID != task.AgentID {
			return nil, pgx.ErrNoRows
		}
		resp.SquadName = squad.Name
		resp.Agent.Instructions += "\n\n" + buildReleaseSquadLeaderBriefing(ctx, h.Queries, squad)
	}
	project, err := h.resolveClaimProjectContext(ctx, repo.ProjectID, pgtype.UUID{}, rel.WorkspaceID, runtime.DaemonID.String)
	if err != nil {
		return nil, err
	}
	project.applyTo(resp)
	// A release starts in its isolated task directory. Repository metadata is
	// available for on-demand checkout, without an issue/worklane branch.
	for i := range resp.Repositories {
		resp.Repositories[i].WorktreePath = ""
	}
	var contextFields map[string]json.RawMessage
	if len(task.Context) > 0 {
		if err := json.Unmarshal(task.Context, &contextFields); err != nil {
			return nil, err
		}
		resp.ReleaseTriggers = contextFields["wakeup_evidence"]
		if len(resp.ReleaseTriggers) == 0 {
			resp.ReleaseTriggers = contextFields["evidence"]
		}
	}
	ids := append([]pgtype.UUID{}, task.CoalescedCommentIds...)
	if task.TriggerCommentID.Valid {
		ids = append(ids, task.TriggerCommentID)
	}
	allowed := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		comment, err := h.Queries.GetCommentInWorkspace(ctx, db.GetCommentInWorkspaceParams{ID: id, WorkspaceID: rel.WorkspaceID})
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		if comment.ReleaseID == rel.ID && !comment.DeletedAt.Valid {
			allowed = append(allowed, id)
		}
	}
	comments := selectCommentDelivery(h.buildCoalescedCommentData(ctx, rel.WorkspaceID, allowed), uuidToString(task.TriggerCommentID), false, maxClaimCommentPayloadBytes)
	resp.CoalescedComments = comments
	resp.CoalescedCommentIDs = uuidStringsOrEmpty(commentDataIDs(comments))
	if trigger := commentByID(comments, uuidToString(task.TriggerCommentID)); trigger.ID != "" {
		resp.TriggerCommentID, resp.TriggerThreadID = uuidToPtr(task.TriggerCommentID), trigger.ThreadID
		resp.TriggerCommentContent, resp.TriggerAuthorType, resp.TriggerAuthorName = trigger.Content, trigger.AuthorType, trigger.AuthorName
	}
	return comments, nil
}
