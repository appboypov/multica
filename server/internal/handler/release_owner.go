package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const releaseActionOwnerChanged = "release_owner_changed"

// RawMessage preserves the distinction between omitted and explicitly null.
type releaseOwnerRequest struct {
	OwnerType json.RawMessage `json:"owner_type"`
	OwnerID   json.RawMessage `json:"owner_id"`
}

func (in releaseOwnerRequest) pair() (pgtype.Text, pgtype.UUID, bool, error) {
	var kind pgtype.Text
	var id pgtype.UUID
	present := len(in.OwnerType) != 0 || len(in.OwnerID) != 0
	if !present {
		return kind, id, false, nil
	}
	if len(in.OwnerType) == 0 || len(in.OwnerID) == 0 {
		return kind, id, true, fmt.Errorf("owner_type and owner_id must be provided together")
	}
	if string(in.OwnerType) == "null" && string(in.OwnerID) == "null" {
		return kind, id, true, nil
	}
	var typ, rawID string
	if json.Unmarshal(in.OwnerType, &typ) != nil || json.Unmarshal(in.OwnerID, &rawID) != nil || (typ != "agent" && typ != "squad") {
		return kind, id, true, fmt.Errorf("owner_type must be 'agent' or 'squad' and owner_id must be a UUID")
	}
	if err := id.Scan(rawID); err != nil || !id.Valid {
		return kind, id, true, fmt.Errorf("invalid owner_id")
	}
	return strToText(typ), id, true, nil
}

type releaseOwnerError struct {
	status  int
	message string
}

func (e *releaseOwnerError) Error() string { return e.message }

func (h *Handler) writeReleaseOwner(ctx context.Context, r *http.Request, q *db.Queries, rel db.Release, in releaseOwnerRequest) error {
	kind, id, present, err := in.pair()
	if err != nil {
		return &releaseOwnerError{http.StatusBadRequest, err.Error()}
	}
	if !present {
		return nil
	}
	if rel.ShippedAt.Valid {
		return &releaseOwnerError{http.StatusConflict, fmt.Sprintf("v%s %s is shipped", rel.Version, rel.Title)}
	}
	if rel.OwnerType == kind && rel.OwnerID == id {
		return nil
	}
	if kind.String == "squad" {
		squad, lockErr := q.LockReleaseOwnerSquad(ctx, db.LockReleaseOwnerSquadParams{ID: id, WorkspaceID: rel.WorkspaceID})
		if lockErr != nil && !errors.Is(lockErr, pgx.ErrNoRows) {
			return lockErr
		}
		if lockErr == nil {
			leader, leaderErr := q.LockReleaseOwnerAgent(ctx, db.LockReleaseOwnerAgentParams{ID: squad.LeaderID, WorkspaceID: rel.WorkspaceID})
			if leaderErr != nil && !errors.Is(leaderErr, pgx.ErrNoRows) {
				return leaderErr
			}
			if leaderErr == nil && leader.Kind == "system" {
				return &releaseOwnerError{http.StatusBadRequest, "squad leader is archived; cannot assign to this squad"}
			}
		}
	} else if kind.String == "agent" {
		agent, lockErr := q.LockReleaseOwnerAgent(ctx, db.LockReleaseOwnerAgentParams{ID: id, WorkspaceID: rel.WorkspaceID})
		if lockErr != nil && !errors.Is(lockErr, pgx.ErrNoRows) {
			return lockErr
		}
		if lockErr == nil && agent.Kind == "system" {
			return &releaseOwnerError{http.StatusBadRequest, "assignee_id does not refer to an agent of this workspace"}
		}
	}
	if status, message := h.validateAssigneePairWithQueries(ctx, r, q, uuidToString(rel.WorkspaceID), kind, id); status != 0 {
		return &releaseOwnerError{status, message}
	}
	previous, err := releaseOwnerSnapshot(ctx, q, rel.WorkspaceID, rel.OwnerType, rel.OwnerID)
	if err != nil {
		return err
	}
	next, err := releaseOwnerSnapshot(ctx, q, rel.WorkspaceID, kind, id)
	if err != nil {
		return err
	}
	updated, err := q.UpdateReleaseOwner(ctx, db.UpdateReleaseOwnerParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID, OwnerType: kind, OwnerID: id})
	if err != nil {
		return err
	}
	cancelled, err := q.CancelUnstartedReleaseTasks(ctx, db.CancelUnstartedReleaseTasksParams{WorkspaceID: rel.WorkspaceID, ReleaseIds: []pgtype.UUID{rel.ID}, Reason: "release owner changed"})
	if err != nil {
		return err
	}
	for _, task := range cancelled {
		if err = service.SettleTerminalTaskState(ctx, q, db.AgentTaskQueue(task)); err != nil {
			return err
		}
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(rel.WorkspaceID))
	details, err := json.Marshal(map[string]any{"release_id": uuidToString(rel.ID), "version": rel.Version, "title": rel.Title,
		"originator_user_id":  h.invokeOriginatorFromRequest(r, actorType, actorID),
		"previous_owner_type": previous["type"], "previous_owner_id": previous["id"], "previous_owner_name": previous["name"],
		"owner_type": next["type"], "owner_id": next["id"], "owner_name": next["name"]})
	if err != nil {
		return err
	}
	if _, err = q.CreateRepositoryActivity(ctx, db.CreateRepositoryActivityParams{WorkspaceID: rel.WorkspaceID, RepositoryID: rel.RepositoryID, ActorType: strToText(actorType), ActorID: parseUUIDOrNull(actorID), Action: releaseActionOwnerChanged, Details: details}); err != nil {
		return err
	}
	return service.CaptureReleaseOwnerChange(ctx, q, updated, actorType, parseUUIDOrNull(actorID), parseUUIDOrNull(h.invokeOriginatorFromRequest(r, actorType, actorID)))
}

func releaseOwnerSnapshot(ctx context.Context, q *db.Queries, ws pgtype.UUID, kind pgtype.Text, id pgtype.UUID) (map[string]any, error) {
	if !kind.Valid || !id.Valid {
		return nil, nil
	}
	name := ""
	if kind.String == "agent" {
		a, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			return nil, err
		}
		name = a.Name
	} else {
		s, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			return nil, err
		}
		name = s.Name
	}
	return map[string]any{"type": kind.String, "id": uuidToString(id), "name": name}, nil
}

func (h *Handler) SetReleaseOwner(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	var in releaseOwnerRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if _, _, present, err := in.pair(); err != nil || !present {
		writeError(w, 400, "owner_type and owner_id must be provided together")
		return
	}
	ctx := r.Context()
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		writeError(w, 500, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		writeMilestoneReadError(w, err, "repository")
		return
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		writeMilestoneReadError(w, err, "repository")
		return
	}
	rel, err = q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID})
	if err != nil {
		writeMilestoneReadError(w, err, "release")
		return
	}
	if err = h.writeReleaseOwner(ctx, r, q, rel, in); err != nil {
		writeReleaseOwnerError(w, err)
		return
	}
	line, err := q.ListReleaseLine(ctx, db.ListReleaseLineParams{WorkspaceID: rel.WorkspaceID, RepositoryID: rel.RepositoryID})
	if err != nil {
		writeError(w, 500, "failed to read release line")
		return
	}
	out, err := h.releasesToResponse(ctx, q, rel.WorkspaceID, line)
	if err != nil {
		writeError(w, 500, "failed to read release owners")
		return
	}
	if err = tx.Commit(ctx); err != nil {
		writeError(w, 500, "failed to commit release owner")
		return
	}
	h.nudgeReleaseCoordination(r.Context(), rel.ID)
	h.publishReleaseLine(r, rel.RepositoryID)
	writeJSON(w, 200, map[string]any{"releases": out})
}

// clearDeletedReleaseOwner removes a deleted owner from its unshipped releases,
// one release line at a time, the way Edit release removes an owner: unstarted
// coordination is cancelled, started runs keep running, and nothing is handed
// to another owner.
func (h *Handler) clearDeletedReleaseOwner(r *http.Request, workspaceID pgtype.UUID, ownerType string, ownerID pgtype.UUID) error {
	ctx := r.Context()
	owned, err := h.Queries.ListUnshippedReleasesOwnedBy(ctx, db.ListUnshippedReleasesOwnedByParams{WorkspaceID: workspaceID, OwnerType: strToText(ownerType), OwnerID: ownerID})
	if err != nil {
		return err
	}
	none := releaseOwnerRequest{OwnerType: json.RawMessage("null"), OwnerID: json.RawMessage("null")}
	for _, rel := range owned {
		tx, err := h.TxStarter.Begin(ctx)
		if err != nil {
			return err
		}
		q := h.Queries.WithTx(tx)
		err = func() error {
			if _, err := q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
				return err
			}
			if _, err := q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
				return err
			}
			current, err := q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID})
			if err != nil {
				return err
			}
			if current.ShippedAt.Valid || current.OwnerType.String != ownerType || current.OwnerID != ownerID {
				return nil
			}
			if err := h.writeReleaseOwner(ctx, r, q, current, none); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}()
		tx.Rollback(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("clear owner of release %s: %w", uuidToString(rel.ID), err)
		}
		slog.Info("release owner cleared after owner deletion", "release_id", uuidToString(rel.ID), "owner_type", ownerType, "owner_id", uuidToString(ownerID))
		h.publishReleaseLine(r, rel.RepositoryID)
	}
	return nil
}

func writeReleaseOwnerError(w http.ResponseWriter, err error) {
	if e, ok := err.(*releaseOwnerError); ok {
		if e.status == http.StatusConflict {
			writeErrorCode(w, e.status, "release_shipped", e.message)
		} else {
			writeError(w, e.status, e.message)
		}
		return
	}
	writeError(w, 500, "failed to write release owner")
}

func (h *Handler) publishReleaseLine(r *http.Request, repositoryID pgtype.UUID) {
	workspaceID := h.resolveWorkspaceID(r)
	actorType, actorID := h.resolveActor(r, requestUserID(r), workspaceID)
	h.publish(protocol.EventReleaseLineUpdated, workspaceID, actorType, actorID, protocol.ReleaseLineEventPayload{RepositoryID: uuidToString(repositoryID)})
}

func (h *Handler) nudgeReleaseCoordination(ctx context.Context, id pgtype.UUID) {
	if h.TaskService == nil {
		return
	}
	if err := (&service.IssueWakeupService{Tasks: h.TaskService}).ProcessReleaseCoordination(ctx, id); err != nil {
		slog.Warn("release coordination dispatch deferred", "release_id", uuidToString(id), "error", err)
	}
}
