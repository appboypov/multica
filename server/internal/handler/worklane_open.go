package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// worklaneOpenRefusal is a reason open cannot save the worklane.
type worklaneOpenRefusal struct {
	code   string
	detail string
}

func (e *worklaneOpenRefusal) Error() string { return e.detail }

// openWorklane is a create request's last step, on the server: it saves the
// worklane with its worktree path and its worklane_created entry in one
// transaction, then completes the request (design decision 3).
func (h *Handler) openWorklane(ctx context.Context, req *WorklaneRequest) {
	step := req.step(worklaneStepOpen)
	if step == nil {
		req.Steps = append(req.Steps, WorklaneRequestStep{Key: worklaneStepOpen})
		step = &req.Steps[len(req.Steps)-1]
	}
	start := time.Now()
	worklane, err := h.saveOpenedWorklane(ctx, req)
	step.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		step.State = worklaneStepFailed
		var refusal *worklaneOpenRefusal
		if errors.As(err, &refusal) {
			step.Detail = refusal.detail
			req.Reason = &WorklaneRequestReason{Code: refusal.code, Detail: refusal.detail}
		} else {
			slog.Error("worklane open failed", "error", err, "request_id", req.ID)
			step.Detail = "the worklane could not be saved"
			req.Reason = nil
		}
		req.Status = worklaneRequestFailed
		return
	}
	step.State = worklaneStepDone
	step.Detail = ""
	id := uuidToString(worklane.ID)
	req.WorklaneID = &id
	req.Status = worklaneRequestCompleted
	req.Reason = nil
	actorID := ""
	if req.CreatedBy != nil {
		actorID = *req.CreatedBy
	}
	h.publishWorklaneEvent(protocol.EventWorklaneCreated, req.WorkspaceID, "member", actorID, worklane)
}

func (h *Handler) saveOpenedWorklane(ctx context.Context, req *WorklaneRequest) (db.Worklane, error) {
	wsUUID := parseUUID(req.WorkspaceID)
	repoUUID := parseUUID(req.RepositoryID)
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.Worklane{}, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)

	if _, err := qtx.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: repoUUID, WorkspaceID: wsUUID}); err != nil {
		if isNotFound(err) {
			return db.Worklane{}, &worklaneOpenRefusal{code: "unknown", detail: "the repository was removed"}
		}
		return db.Worklane{}, err
	}
	// The name and folder again: another worklane may have taken them while
	// the steps ran.
	all, err := qtx.ListWorklanes(ctx, wsUUID)
	if err != nil {
		return db.Worklane{}, err
	}
	for _, wl := range all {
		if wl.RepositoryID != repoUUID {
			continue
		}
		if strings.EqualFold(wl.Name, req.Name) {
			return db.Worklane{}, &worklaneOpenRefusal{code: "name_taken", detail: fmt.Sprintf("%s is taken in this repository", wl.Name)}
		}
		if wl.Folder.Valid && wl.Folder.String == req.Folder {
			return db.Worklane{}, &worklaneOpenRefusal{code: "folder_taken", detail: fmt.Sprintf("%s already uses this folder", wl.Name)}
		}
	}
	worklane, err := qtx.CreateWorklaneWithPath(ctx, db.CreateWorklaneWithPathParams{
		WorkspaceID:  wsUUID,
		RepositoryID: repoUUID,
		Name:         req.Name,
		Folder:       strToText(req.Folder),
		DaemonID:     strToText(req.DaemonID),
		Path:         strToText(req.Path),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "worklane_repository_folder_unique" {
				return db.Worklane{}, &worklaneOpenRefusal{code: "folder_taken", detail: "another worklane already uses this folder"}
			}
			return db.Worklane{}, &worklaneOpenRefusal{code: "name_taken", detail: fmt.Sprintf("%s is taken in this repository", req.Name)}
		}
		return db.Worklane{}, err
	}
	details, err := json.Marshal(map[string]string{"name": req.Name, "runtime_name": req.RuntimeName})
	if err != nil {
		return db.Worklane{}, err
	}
	var actorID pgtype.UUID
	if req.CreatedBy != nil {
		if u, err := util.ParseUUID(*req.CreatedBy); err == nil {
			actorID = u
		}
	}
	if _, err := qtx.CreateWorklaneActivity(ctx, db.CreateWorklaneActivityParams{
		WorkspaceID:  wsUUID,
		WorklaneID:   worklane.ID,
		RepositoryID: repoUUID,
		ActorType:    strToText("member"),
		ActorID:      actorID,
		Action:       worklaneActionCreated,
		Details:      details,
	}); err != nil {
		return db.Worklane{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Worklane{}, err
	}
	return worklane, nil
}
