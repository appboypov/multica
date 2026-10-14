package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/releaseline"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Release activity actions (ADR-0008, ADR-0011). Each release line write adds
// one activity_log row with repository_id set and worklane_id and issue_id
// NULL. Details carry {release_id, version, title}, plus changes on edit,
// move and remove, and direction on move.
const (
	releaseActionPlanned = "release_planned"
	releaseActionEdited  = "release_edited"
	releaseActionMoved   = "release_moved"
	releaseActionRemoved = "release_removed"
)

// Release refusal codes (design.md decision 2), checked in this order inside
// the transaction, after the 404s.
const (
	releaseCodeTitleRequired  = "title_required"
	releaseCodeVersionOrBump  = "version_or_bump"
	releaseCodeVersionInvalid = "version_invalid"
	releaseCodeDoesNotRelease = "does_not_release"
	releaseCodeLastTagUnknown = "last_tag_unknown"
)

// versionPolicyNone is the version policy of a repository that does not
// release.
const versionPolicyNone = "none"

// ReleaseResponse is one release of a repository's line. Version has no
// leading `v`; a null bump means the version was set by hand.
type ReleaseResponse struct {
	ID           string  `json:"id"`
	RepositoryID string  `json:"repository_id"`
	Position     int32   `json:"position"`
	Version      string  `json:"version"`
	Bump         *string `json:"bump"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	ShippedAt    *string `json:"shipped_at"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func releaseToResponse(r db.Release) ReleaseResponse {
	return ReleaseResponse{
		ID:           uuidToString(r.ID),
		RepositoryID: uuidToString(r.RepositoryID),
		Position:     r.Position,
		Version:      r.Version,
		Bump:         textToPtr(r.Bump),
		Title:        r.Title,
		Description:  r.Description,
		ShippedAt:    timestampToPtr(r.ShippedAt),
		CreatedAt:    timestampToString(r.CreatedAt),
		UpdatedAt:    timestampToString(r.UpdatedAt),
	}
}

func releasesToResponse(rows []db.Release) []ReleaseResponse {
	out := make([]ReleaseResponse, len(rows))
	for i, r := range rows {
		out[i] = releaseToResponse(r)
	}
	return out
}

// releaseLineOf is the line releaseline works on: the repository's last tag
// and its releases by position.
func releaseLineOf(repo db.Repository, rows []db.Release) releaseline.Line {
	line := releaseline.Line{LastTag: repo.LastTag.String, Releases: make([]releaseline.Release, len(rows))}
	for i, r := range rows {
		line.Releases[i] = releaseline.Release{
			ID:      uuidToString(r.ID),
			Version: r.Version,
			Bump:    r.Bump.String,
			Title:   r.Title,
			Shipped: r.ShippedAt.Valid,
		}
	}
	return line
}

// ListReleases answers every release of the workspace, by repository and
// position (GET /api/releases).
func (h *Handler) ListReleases(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	rows, err := h.Queries.ListReleasesInWorkspace(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list releases")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": releasesToResponse(rows)})
}

// releaseRequest is the body of Plan release and Edit release: a title, a
// description, and either a bump or a version.
type releaseRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Bump        *string  `json:"bump"`
	Version     *string  `json:"version"`
	IssueIDs    []string `json:"issue_ids"`
}

// releaseFields are a validated releaseRequest: Bump is "" when Version was
// set by hand, and Version is "" when Bump computes it.
type releaseFields struct {
	Title       string
	Description string
	Bump        string
	Version     string
}

// validate answers the body refusals in table order and reports whether the
// body passed.
func (req releaseRequest) validate(w http.ResponseWriter) (releaseFields, bool) {
	var f releaseFields
	if req.Title != nil {
		f.Title = strings.TrimSpace(*req.Title)
	}
	if f.Title == "" {
		writeErrorCode(w, http.StatusBadRequest, releaseCodeTitleRequired, "title is required")
		return f, false
	}
	if req.Description != nil {
		f.Description = *req.Description
	}
	if (req.Bump == nil) == (req.Version == nil) {
		writeErrorCode(w, http.StatusBadRequest, releaseCodeVersionOrBump, "send either bump or version")
		return f, false
	}
	if req.Bump != nil {
		if !releaseline.ValidBump(*req.Bump) {
			writeErrorCode(w, http.StatusBadRequest, releaseCodeVersionOrBump, "bump must be patch, minor or major")
			return f, false
		}
		f.Bump = *req.Bump
		return f, true
	}
	v, ok := releaseline.ParseVersion(*req.Version)
	if !ok {
		writeErrorCode(w, http.StatusBadRequest, releaseCodeVersionInvalid, "use the form v1.4.0")
		return f, false
	}
	f.Version = v.String()
	return f, true
}

// releaseLineWrite is one change to a repository's release line.
type releaseLineWrite struct {
	// releaseID is the release the change is about, or invalid for a plan.
	releaseID pgtype.UUID

	// validate answers the body refusals; nil when the change has no body.
	validate func(w http.ResponseWriter) bool

	// change runs the releaseline operation on the line.
	change func(line releaseline.Line) (releaseline.Line, *releaseline.Problem)

	// write inserts, updates or deletes the release itself and answers the
	// activity details.
	write       func(ctx context.Context, qtx *db.Queries, repo db.Repository, before, after releaseline.Line) (map[string]any, error)
	afterCommit func()

	// wroteVersion is true when write already set the version of releaseID,
	// which keeps its position, so the runner leaves it out when it rewrites
	// the positions and versions of the rest of the line.
	wroteVersion bool

	// action is the activity action and status the answer's status.
	action string
	status int
}

// runReleaseLineWrite runs one release line write as design.md decision 3
// sets out: lock the repository, refuse, change the line, write it, add one
// activity row, commit, publish release_line:updated and answer the line.
func (h *Handler) runReleaseLineWrite(w http.ResponseWriter, r *http.Request, repositoryID pgtype.UUID, op releaseLineWrite) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)

	repo, err := qtx.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: repositoryID, WorkspaceID: wsUUID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to lock repository")
		return
	}
	rows, err := qtx.ListReleaseLine(ctx, db.ListReleaseLineParams{WorkspaceID: wsUUID, RepositoryID: repo.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read release line")
		return
	}
	if op.releaseID.Valid && !releaseOnLine(rows, op.releaseID) {
		writeError(w, http.StatusNotFound, "release not found")
		return
	}
	if op.validate != nil && !op.validate(w) {
		return
	}
	if repo.VersionPolicy == versionPolicyNone {
		writeErrorCode(w, http.StatusConflict, releaseCodeDoesNotRelease, "this repository does not release")
		return
	}
	if !repo.LastTag.Valid {
		writeErrorCode(w, http.StatusConflict, releaseCodeLastTagUnknown, "the last tag of this repository is not known yet")
		return
	}

	before := releaseLineOf(repo, rows)
	after, problem := op.change(before)
	if problem != nil {
		writeReleaseProblem(w, problem)
		return
	}
	details, err := op.write(ctx, qtx, repo, before, after)
	if err != nil {
		if writePlacementError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to write release")
		return
	}
	skip := pgtype.UUID{}
	if op.wroteVersion {
		skip = op.releaseID
	}
	if err := rewriteReleaseLine(ctx, qtx, wsUUID, skip, before, after); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to renumber release line")
		return
	}

	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode release activity")
		return
	}
	if _, err := qtx.CreateRepositoryActivity(ctx, db.CreateRepositoryActivityParams{
		WorkspaceID:  wsUUID,
		RepositoryID: repo.ID,
		ActorType:    strToText(actorType),
		ActorID:      parseUUIDOrNull(actorID),
		Action:       op.action,
		Details:      detailsJSON,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record release activity")
		return
	}

	line, err := qtx.ListReleaseLine(ctx, db.ListReleaseLineParams{WorkspaceID: wsUUID, RepositoryID: repo.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read release line")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit release line")
		return
	}
	if op.afterCommit != nil {
		op.afterCommit()
	}
	h.publish(protocol.EventReleaseLineUpdated, workspaceID, actorType, actorID, protocol.ReleaseLineEventPayload{
		RepositoryID: uuidToString(repo.ID),
	})
	writeJSON(w, op.status, map[string]any{"releases": releasesToResponse(line)})
}

// rewriteReleaseLine writes the position and version of every release of
// after whose place or version changed, except skip, which the operation
// already wrote, and a planned release, which has no id yet.
func rewriteReleaseLine(ctx context.Context, qtx *db.Queries, wsUUID, skip pgtype.UUID, before, after releaseline.Line) error {
	type place struct {
		position int
		version  string
	}
	old := make(map[string]place, len(before.Releases))
	for i, rel := range before.Releases {
		old[rel.ID] = place{position: i, version: rel.Version}
	}
	skipID := ""
	if skip.Valid {
		skipID = uuidToString(skip)
	}
	for i, rel := range after.Releases {
		if rel.ID == "" || rel.ID == skipID {
			continue
		}
		if p, ok := old[rel.ID]; ok && p.position == i && p.version == rel.Version {
			continue
		}
		if err := qtx.UpdateReleasePositionAndVersion(ctx, db.UpdateReleasePositionAndVersionParams{
			ID:          parseUUID(rel.ID),
			WorkspaceID: wsUUID,
			Position:    int32(i),
			Version:     rel.Version,
		}); err != nil {
			return err
		}
	}
	return nil
}

func releaseOnLine(rows []db.Release, id pgtype.UUID) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// writeReleaseProblem answers a releaseline problem. line_not_increasing
// carries the release and what stands before it: {version, title} of the
// release before it, or {last_tag}.
func writeReleaseProblem(w http.ResponseWriter, p *releaseline.Problem) {
	switch p.Code {
	case releaseline.CodeNotOnLine:
		writeError(w, http.StatusNotFound, "release not found")
	case releaseline.CodeFirstRelease:
		writeErrorCode(w, http.StatusConflict, p.Code, "it is the first release")
	case releaseline.CodeLastRelease:
		writeErrorCode(w, http.StatusConflict, p.Code, "it is the last release")
	default:
		before := map[string]string{"last_tag": p.LastTag}
		if p.Before != nil {
			before = map[string]string{"version": p.Before.Version, "title": p.Before.Title}
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":      "the line would not be strictly increasing",
			"code":       releaseline.CodeLineNotIncreasing,
			"release_id": p.Release.ID,
			"version":    p.Release.Version,
			"title":      p.Release.Title,
			"before":     before,
		})
	}
}

// parseUUIDOrNull reads a server-trusted id, such as the actor from
// resolveActor, as NULL when it is not a UUID.
func parseUUIDOrNull(s string) pgtype.UUID {
	u, err := util.ParseUUID(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return u
}

// lineRelease finds the release id on a line.
func lineRelease(line releaseline.Line, id string) releaseline.Release {
	for _, r := range line.Releases {
		if r.ID == id {
			return r
		}
	}
	return releaseline.Release{ID: id}
}

// loadRelease reads the release of the URL's id in the workspace, answering
// 404 when it is not there.
func (h *Handler) loadRelease(w http.ResponseWriter, r *http.Request) (db.Release, bool) {
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "release id")
	if !ok {
		return db.Release{}, false
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return db.Release{}, false
	}
	rel, err := h.Queries.GetReleaseInWorkspace(r.Context(), db.GetReleaseInWorkspaceParams{ID: id, WorkspaceID: wsUUID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "release not found")
			return db.Release{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load release")
		return db.Release{}, false
	}
	return rel, true
}

// PlanRelease adds a release at the end of a repository's line
// (POST /api/repositories/{id}/releases).
func (h *Handler) PlanRelease(w http.ResponseWriter, r *http.Request) {
	repositoryID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "repository id")
	if !ok {
		return
	}
	var req releaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var fields releaseFields
	userUUID := parseUUIDOrNull(requestUserID(r))
	ids, err := parseIssuePlacementIDs(req.IssueIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), h.resolveWorkspaceID(r))
	var issues []db.Issue
	var activities []db.ActivityLog
	h.runReleaseLineWrite(w, r, repositoryID, releaseLineWrite{
		validate: func(w http.ResponseWriter) bool {
			var ok bool
			fields, ok = req.validate(w)
			return ok
		},
		change: func(line releaseline.Line) (releaseline.Line, *releaseline.Problem) {
			return releaseline.Plan(line, releaseline.Release{Version: fields.Version, Bump: fields.Bump, Title: fields.Title})
		},
		write: func(ctx context.Context, qtx *db.Queries, repo db.Repository, before, after releaseline.Line) (map[string]any, error) {
			planned := after.Releases[len(after.Releases)-1]
			row, err := qtx.CreateRelease(ctx, db.CreateReleaseParams{
				WorkspaceID:  repo.WorkspaceID,
				RepositoryID: repo.ID,
				Position:     int32(len(after.Releases) - 1),
				Version:      planned.Version,
				Bump:         bumpText(fields.Bump),
				Title:        fields.Title,
				Description:  fields.Description,
				CreatedBy:    userUUID,
			})
			if err != nil {
				return nil, err
			}
			issues, activities, err = h.placeIssuesInRelease(ctx, qtx, repo, row, ids, actorType, actorID)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"release_id": uuidToString(row.ID),
				"version":    row.Version,
				"title":      row.Title,
			}, nil
		},
		afterCommit: func() {
			h.publishIssueSnapshots(r.Context(), issues, actorType, actorID)
			service.PublishIssueActivities(h.Bus, parseUUIDOrNull(h.resolveWorkspaceID(r)), actorType, actorID, activities)
		},
		action: releaseActionPlanned,
		status: http.StatusCreated,
	})
}

// EditRelease changes a release's version, title and description in place
// (PATCH /api/releases/{id}).
func (h *Handler) EditRelease(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	var req releaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	id := uuidToString(rel.ID)
	var fields releaseFields
	h.runReleaseLineWrite(w, r, rel.RepositoryID, releaseLineWrite{
		releaseID: rel.ID,
		validate: func(w http.ResponseWriter) bool {
			var ok bool
			fields, ok = req.validate(w)
			return ok
		},
		change: func(line releaseline.Line) (releaseline.Line, *releaseline.Problem) {
			return releaseline.Edit(line, id, releaseline.Release{Version: fields.Version, Bump: fields.Bump, Title: fields.Title})
		},
		write: func(ctx context.Context, qtx *db.Queries, repo db.Repository, before, after releaseline.Line) (map[string]any, error) {
			edited := lineRelease(after, id)
			row, err := qtx.UpdateReleaseFields(ctx, db.UpdateReleaseFieldsParams{
				ID:          rel.ID,
				WorkspaceID: rel.WorkspaceID,
				Title:       fields.Title,
				Description: fields.Description,
				Version:     edited.Version,
				Bump:        bumpText(fields.Bump),
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"release_id": id,
				"version":    row.Version,
				"title":      row.Title,
				"changes":    releaseline.Changes(before, after),
			}, nil
		},
		wroteVersion: true,
		action:       releaseActionEdited,
		status:       http.StatusOK,
	})
}

// MoveRelease swaps a release with its neighbour on the line
// (POST /api/releases/{id}/move).
func (h *Handler) MoveRelease(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	var req struct {
		Direction string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Direction != releaseline.Earlier && req.Direction != releaseline.Later {
		writeError(w, http.StatusBadRequest, "direction must be earlier or later")
		return
	}
	id := uuidToString(rel.ID)
	h.runReleaseLineWrite(w, r, rel.RepositoryID, releaseLineWrite{
		releaseID: rel.ID,
		change: func(line releaseline.Line) (releaseline.Line, *releaseline.Problem) {
			return releaseline.Move(line, id, req.Direction)
		},
		write: func(ctx context.Context, qtx *db.Queries, repo db.Repository, before, after releaseline.Line) (map[string]any, error) {
			// The runner writes the new places of the whole line, the moved
			// release's included.
			moved := lineRelease(after, id)
			return map[string]any{
				"release_id": id,
				"version":    moved.Version,
				"title":      moved.Title,
				"direction":  req.Direction,
				"changes":    releaseline.Changes(before, after),
			}, nil
		},
		action: releaseActionMoved,
		status: http.StatusOK,
	})
}

// RemoveRelease takes an unshipped release off the line
// (DELETE /api/releases/{id}).
func (h *Handler) RemoveRelease(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	id := uuidToString(rel.ID)
	actorType, actorID := h.resolveActor(r, requestUserID(r), h.resolveWorkspaceID(r))
	var issues []db.Issue
	var activities []db.ActivityLog
	h.runReleaseLineWrite(w, r, rel.RepositoryID, releaseLineWrite{
		releaseID: rel.ID,
		change: func(line releaseline.Line) (releaseline.Line, *releaseline.Problem) {
			return releaseline.Remove(line, id)
		},
		write: func(ctx context.Context, qtx *db.Queries, repo db.Repository, before, after releaseline.Line) (map[string]any, error) {
			removed := lineRelease(before, id)
			ids, err := qtx.ClearReleaseOfIssues(ctx, db.ClearReleaseOfIssuesParams{WorkspaceID: repo.WorkspaceID, ReleaseID: rel.ID})
			if err != nil {
				return nil, err
			}
			current := rel
			current.Version = removed.Version
			current.Title = removed.Title
			issues, activities, err = h.releaseIssueEntries(ctx, qtx, repo.WorkspaceID, ids, actorType, actorID, issueActionReleaseGone, current)
			if err != nil {
				return nil, err
			}
			if err := qtx.DeleteRelease(ctx, db.DeleteReleaseParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID}); err != nil {
				return nil, err
			}
			return map[string]any{
				"release_id": id,
				"version":    removed.Version,
				"title":      removed.Title,
				"changes":    releaseline.Changes(before, after),
			}, nil
		},
		afterCommit: func() {
			h.publishIssueSnapshots(r.Context(), issues, actorType, actorID)
			service.PublishIssueActivities(h.Bus, rel.WorkspaceID, actorType, actorID, activities)
		},
		action: releaseActionRemoved,
		status: http.StatusOK,
	})
}

// bumpText stores a bump, or NULL for a version set by hand.
func bumpText(bump string) pgtype.Text {
	if bump == "" {
		return pgtype.Text{}
	}
	return strToText(bump)
}
