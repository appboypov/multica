package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) enqueueWorklaneSwitch(ctx context.Context, sw service.WorklaneSwitch) error {
	machine, err := h.loadWorklaneMachine(ctx, sw.Worklane.WorkspaceID, sw.DaemonID)
	if err != nil {
		return err
	}
	id := uuidToString(sw.Worklane.ID)
	req := &WorklaneRequest{
		ID: randomID(), Kind: protocol.WorklaneRequestKindSwitch,
		WorkspaceID: uuidToString(sw.Worklane.WorkspaceID), RepositoryID: uuidToString(sw.Repository.ID),
		WorklaneID: &id, Name: sw.Worklane.Name, Path: sw.Path,
		DaemonID: sw.DaemonID, RuntimeName: machine.name,
		FullName: sw.Repository.FullName, CheckoutPath: sw.Repository.CheckoutPath,
		DefaultBranch: sw.DefaultBranch, FromBranch: sw.FromBranch, ToBranch: sw.ToBranch,
		Status: worklaneRequestRunning, CreatedAt: time.Now(),
		Steps: []WorklaneRequestStep{
			{Key: "check", State: worklaneStepPending},
			{Key: "fetch", State: worklaneStepPending},
			{Key: "switch", State: worklaneStepPending},
		},
	}
	// A lost machine leaves a failed request, not a switch on another machine.
	if !machine.online {
		req.Status = worklaneRequestFailed
		req.Reason = &WorklaneRequestReason{Code: "lost", Detail: machine.name + " stopped answering"}
		req.Steps[0].State = worklaneStepFailed
		req.Steps[0].Detail = req.Reason.Detail
		req.UpdatedAt = time.Now()
		return h.WorklaneRequestStore.Save(ctx, req)
	}
	return h.enqueueWorklaneRequest(ctx, req, uuidToString(machine.runtime.ID))
}

func (h *Handler) finishWorklaneSwitch(ctx context.Context, req *WorklaneRequest, steps []WorklaneRequestStep, outcome, branch string, reason *WorklaneRequestReason) error {
	for _, step := range steps {
		if existing := req.step(step.Key); existing != nil {
			*existing = step
		}
	}
	if outcome == "" {
		return nil
	}
	if outcome == worklaneOutcomeFailed {
		req.Status, req.Reason = worklaneRequestFailed, reason
		return nil
	}
	if req.WorklaneID == nil {
		return fmt.Errorf("switch request has no worklane")
	}
	wl, err := h.Queries.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: parseUUID(*req.WorklaneID), WorkspaceID: parseUUID(req.WorkspaceID)})
	if err != nil {
		return err
	}
	switch outcome {
	case "completed":
		if branch != req.ToBranch {
			return fmt.Errorf("switch reported an unexpected branch")
		}
		var issueID pgtype.UUID
		if branch != "" {
			rows, err := h.Queries.ListOpenWorklaneIssues(ctx, db.ListOpenWorklaneIssuesParams{WorkspaceID: wl.WorkspaceID, WorklaneIds: []pgtype.UUID{wl.ID}})
			if err != nil {
				return err
			}
			for _, row := range rows {
				if branch == strings.ToLower(row.Identifier) {
					issueID = row.ID
					break
				}
			}
		}
		if _, err := h.recordWorklaneBranch(ctx, wl, pgtype.Text{String: branch, Valid: branch != ""}, issueID, "system", pgtype.UUID{}); err != nil {
			return err
		}
		req.Status, req.Reason = worklaneRequestCompleted, nil
	case worklaneOutcomeRefused:
		if reason == nil || (reason.Code != "uncommitted" && reason.Code != "unpushed") {
			return fmt.Errorf("switch refusal has no valid reason")
		}
		count := 0
		if reason.Count != nil {
			count = *reason.Count
		}
		if err := h.recordWorklaneKept(ctx, wl, reason.Code, count, "system", pgtype.UUID{}); err != nil {
			return err
		}
		req.Status, req.Reason = worklaneRequestRefused, reason
	default:
		return fmt.Errorf("unknown switch outcome %q", outcome)
	}
	return nil
}
