package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workpath"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// workRootOf reads the workspace's work root from its settings, the default
// when they name none (ADR-0037).
func (h *Handler) workRootOf(ctx context.Context, workspaceID pgtype.UUID) (string, error) {
	ws, err := h.Queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return workpath.RootFromSettings(ws.Settings), nil
}

// projectFolderOf is where a project's repositories live, read now: the work
// root, the key of the project's organisation and the project's slug. A
// missing organisation or slug stays empty; workpath refuses to build a path
// from it.
func (h *Handler) projectFolderOf(ctx context.Context, workspaceID, projectID pgtype.UUID) (workpath.Project, error) {
	root, err := h.workRootOf(ctx, workspaceID)
	if err != nil {
		return workpath.Project{}, err
	}
	project, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: projectID, WorkspaceID: workspaceID})
	if err != nil {
		return workpath.Project{}, err
	}
	folder := workpath.Project{Root: root, Slug: project.Slug.String}
	if !project.OrganisationID.Valid {
		return folder, nil
	}
	org, err := h.Queries.GetOrganisationInWorkspace(ctx, db.GetOrganisationInWorkspaceParams{ID: project.OrganisationID, WorkspaceID: workspaceID})
	if isNotFound(err) {
		return folder, nil
	}
	if err != nil {
		return workpath.Project{}, err
	}
	folder.Organisation = org.Key
	return folder, nil
}
