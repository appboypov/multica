package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/issueplacement"
	"github.com/multica-ai/multica/server/internal/issuetemplate"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// createPlacement is what a new issue's placement resolves to: the project
// its worklane sets, invalid when it has none, and the entries to write.
type createPlacement struct {
	projectID pgtype.UUID
	entries   []issueplacement.Entry
}

// resolveCreatePlacement locks the worklane's repository, looks up the
// worklane and the release and applies the placement rules (ADR-0013).
func resolveCreatePlacement(ctx context.Context, qtx *db.Queries, p IssueCreateParams) (createPlacement, error) {
	if !p.WorklaneID.Valid && !p.ReleaseID.Valid {
		return createPlacement{}, validateCreateIssueType(p)
	}
	in := issueplacement.Input{
		Creating: true,
		Want: issueplacement.Placement{
			WorklaneID: util.UUIDToString(p.WorklaneID),
			ReleaseID:  util.UUIDToString(p.ReleaseID),
		},
	}
	var projectID pgtype.UUID
	if p.WorklaneID.Valid {
		wl, err := qtx.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: p.WorklaneID, WorkspaceID: p.WorkspaceID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return createPlacement{}, ErrWorklaneNotFound
			}
			return createPlacement{}, fmt.Errorf("load worklane: %w", err)
		}
		repo, err := qtx.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: wl.RepositoryID, WorkspaceID: p.WorkspaceID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return createPlacement{}, ErrWorklaneNotFound
			}
			return createPlacement{}, fmt.Errorf("lock repository: %w", err)
		}
		// A removal may have committed while this create waited for the line lock.
		wl, err = qtx.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: p.WorklaneID, WorkspaceID: p.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) { return createPlacement{}, ErrWorklaneNotFound }
		if err != nil { return createPlacement{}, fmt.Errorf("reload worklane: %w", err) }
		projectID = repo.ProjectID
		in.Worklane = &issueplacement.Worklane{
			ID:           util.UUIDToString(wl.ID),
			RepositoryID: util.UUIDToString(repo.ID),
			ProjectID:    util.UUIDToString(repo.ProjectID),
			Name:         wl.Name,
		}
	}
	if p.ReleaseID.Valid {
		rel, err := LoadPlacementRelease(ctx, qtx, p.WorkspaceID, p.ReleaseID)
		if err != nil {
			return createPlacement{}, err
		}
		in.Release = rel
	}
	if err := validateCreateIssueType(p); err != nil { return createPlacement{},err }
	result, refusal := issueplacement.Resolve(in)
	if refusal != nil {
		return createPlacement{}, &IssuePlacementRefusal{Refusal: *refusal}
	}
	return createPlacement{projectID: projectID, entries: result.Entries}, nil
}

// LoadPlacementWorklane looks up a worklane and its repository's project in
// the workspace, answering ErrWorklaneNotFound when either is not there.
func LoadPlacementWorklane(ctx context.Context, q *db.Queries, workspaceID, id pgtype.UUID) (*issueplacement.Worklane, error) {
	wl, err := q.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorklaneNotFound
		}
		return nil, fmt.Errorf("load worklane: %w", err)
	}
	repo, err := q.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: wl.RepositoryID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorklaneNotFound
		}
		return nil, fmt.Errorf("load worklane repository: %w", err)
	}
	return &issueplacement.Worklane{
		ID:           util.UUIDToString(wl.ID),
		RepositoryID: util.UUIDToString(repo.ID),
		ProjectID:    util.UUIDToString(repo.ProjectID),
		Name:         wl.Name,
	}, nil
}

// LoadPlacementRelease looks up a release and its repository's name in the
// workspace, answering ErrReleaseNotFound when either is not there.
func LoadPlacementRelease(ctx context.Context, q *db.Queries, workspaceID, id pgtype.UUID) (*issueplacement.Release, error) {
	rel, err := q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReleaseNotFound
		}
		return nil, fmt.Errorf("load release: %w", err)
	}
	repo, err := q.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: rel.RepositoryID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReleaseNotFound
		}
		return nil, fmt.Errorf("load release repository: %w", err)
	}
	return PlacementRelease(rel, repo.FullName), nil
}

// PlacementRelease is a release row as the placement rules read it.
func PlacementRelease(rel db.Release, repositoryName string) *issueplacement.Release {
	return &issueplacement.Release{
		ID:             util.UUIDToString(rel.ID),
		RepositoryID:   util.UUIDToString(rel.RepositoryID),
		RepositoryName: repositoryName,
		Version:        rel.Version,
		Title:          rel.Title,
		Shipped:        rel.ShippedAt.Valid,
	}
}

// createActorID is the actor of a create's entries: the acting agent when
// the caller named one, else the creator.
func createActorID(p IssueCreateParams, opts IssueCreateOpts) string {
	if opts.ActorID != "" {
		return opts.ActorID
	}
	return util.UUIDToString(p.CreatorID)
}

// WriteIssueEntries adds one activity_log row per entry to an issue, in the
// caller's transaction, with the actor of the write.
func WriteIssueEntries(ctx context.Context, q *db.Queries, workspaceID, issueID pgtype.UUID, actorType, actorID string, entries []issueplacement.Entry) ([]db.ActivityLog, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	actor, err := util.ParseUUID(actorID)
	if err != nil {
		actor = pgtype.UUID{}
	}
	out := make([]db.ActivityLog, 0, len(entries))
	for _, e := range entries {
		details, err := json.Marshal(e.Details)
		if err != nil {
			return nil, fmt.Errorf("encode %s details: %w", e.Action, err)
		}
		row, err := q.CreateActivity(ctx, db.CreateActivityParams{
			ID:          dbid.NewV7(),
			WorkspaceID: workspaceID,
			IssueID:     issueID,
			ActorType:   util.StrToText(actorType),
			ActorID:     actor,
			Action:      e.Action,
			Details:     details,
		})
		if err != nil {
			return nil, fmt.Errorf("record %s: %w", e.Action, err)
		}
		out = append(out, row)
	}
	return out, nil
}

// PublishIssueActivities publishes activity:created for entries a write
// committed, in the shape the issue:updated activity listener publishes.
func PublishIssueActivities(bus *events.Bus, workspaceID pgtype.UUID, actorType, actorID string, activities []db.ActivityLog) {
	if bus == nil {
		return
	}
	for _, a := range activities {
		entryActorType := ""
		if a.ActorType.Valid {
			entryActorType = a.ActorType.String
		}
		bus.Publish(events.Event{
			Type:        protocol.EventActivityCreated,
			WorkspaceID: util.UUIDToString(workspaceID),
			ActorType:   actorType,
			ActorID:     actorID,
			Payload: map[string]any{
				"issue_id": util.UUIDToString(a.IssueID),
				"entry": map[string]any{
					"type":       "activity",
					"id":         util.UUIDToString(a.ID),
					"actor_type": entryActorType,
					"actor_id":   util.UUIDToString(a.ActorID),
					"action":     a.Action,
					"details":    json.RawMessage(a.Details),
					"created_at": util.TimestampToString(a.CreatedAt),
				},
			},
		})
	}
}

func validateCreateIssueType(p IssueCreateParams) error {
 if (p.Type.Valid && !issuetemplate.IsType(p.Type.String)) || (p.FromTemplate && !p.Type.Valid) { return ErrIssueTypeInvalid }
 return nil
}
