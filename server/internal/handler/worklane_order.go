package handler

import (
	"encoding/json"
	"math"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// MoveWorklaneIssue changes only the lane order, never the board position or release.
func (h *Handler) MoveWorklaneIssue(w http.ResponseWriter, r *http.Request) {
	lane, ws, ok := h.loadWorkspaceWorklane(w, r)
	if !ok {
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "issueId"))
	if !ok {
		return
	}
	if issue.WorkspaceID != ws || issue.WorklaneID != lane.ID {
		writeError(w, http.StatusNotFound, "issue not found in this worklane")
		return
	}
	var body struct {
		Before string `json:"before"`
		After  string `json:"after"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || (body.Before == "") == (body.After == "") {
		writeError(w, http.StatusBadRequest, "provide exactly one of before or after")
		return
	}
	targetID := body.Before
	if targetID == "" {
		targetID = body.After
	}
	target, ok := parseUUIDOrBadRequest(w, targetID, "target issue id")
	if !ok {
		return
	}
	if target == issue.ID {
		writeError(w, http.StatusBadRequest, "cannot move an issue relative to itself")
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
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: lane.RepositoryID, WorkspaceID: ws}); err != nil {
		writeError(w, 404, "repository not found")
		return
	}
	rows, err := q.ListWorklaneOrder(ctx, db.ListWorklaneOrderParams{WorkspaceID: ws, WorklaneID: lane.ID})
	if err != nil {
		writeError(w, 500, "failed to load lane order")
		return
	}
	var moving, other *db.ListWorklaneOrderRow
	for i := range rows {
		if rows[i].ID == issue.ID {
			moving = &rows[i]
		}
		if rows[i].ID == target {
			other = &rows[i]
		}
	}
	if moving == nil || other == nil {
		writeError(w, 404, "issue not found in this worklane")
		return
	}
	if moving.ReleaseID != other.ReleaseID {
		writeError(w, http.StatusConflict, "issues in different releases keep release order")
		return
	}
	ordered := make([]db.ListWorklaneOrderRow, 0, len(rows))
	for _, row := range rows {
		if row.ReleaseID != moving.ReleaseID || row.ID == moving.ID {
			continue
		}
		if row.ID == target && body.Before != "" {
			ordered = append(ordered, *moving)
		}
		ordered = append(ordered, row)
		if row.ID == target && body.After != "" {
			ordered = append(ordered, *moving)
		}
	}
	position, fractional := laneMovePosition(ordered, moving.ID)
	changed := []db.Issue{}
	for i, row := range ordered {
		if fractional && row.ID != moving.ID {
			continue
		}
		value := position
		if !fractional {
			value = float64(i + 1)
		}
		updated, err := q.SetIssueLanePosition(ctx, db.SetIssueLanePositionParams{ID: row.ID, WorkspaceID: ws, WorklaneID: lane.ID, LanePosition: pgtype.Float8{Float64: value, Valid: true}})
		if err != nil {
			writeError(w, 500, "failed to move issue")
			return
		}
		changed = append(changed, updated)
	}
	if tx.Commit(ctx) != nil {
		writeError(w, 500, "failed to commit lane order")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(ws))
	h.publishIssueSnapshots(ctx, changed, actorType, actorID)
	h.publishWorklaneUpdated(ctx, lane, actorType, actorID)
	h.ListWorklaneIssues(w, r)
}

// Fractional positions leave room between neighbors. Equal, null or exhausted
// neighbors are normalized in the same transaction rather than losing the move.
func laneMovePosition(rows []db.ListWorklaneOrderRow, id pgtype.UUID) (float64, bool) {
	for i, row := range rows {
		if row.ID != id {
			continue
		}
		var left, right float64
		if i > 0 {
			if !rows[i-1].LanePosition.Valid {
				return 0, false
			}
			left = rows[i-1].LanePosition.Float64
		}
		if i+1 < len(rows) {
			if !rows[i+1].LanePosition.Valid {
				return 0, false
			}
			right = rows[i+1].LanePosition.Float64
		}
		position := left + 1
		if i == 0 && len(rows) > 1 {
			position = right - 1
		} else if i+1 < len(rows) {
			position = left + (right-left)/2
		}
		return position, !math.IsNaN(position) && !math.IsInf(position, 0) && (i == 0 || position > left) && (i+1 == len(rows) || position < right)
	}
	return 0, false
}
