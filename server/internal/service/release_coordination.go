package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

var ErrReleaseOwnerUnavailable = errors.New("release owner is not runnable")

// ResolveReleaseTaskOwner resolves the current leader, never a stored leader snapshot.
// Callers making a queue decision hold the release's repository boundary.
func ResolveReleaseTaskOwner(ctx context.Context, q *db.Queries, release db.Release) (db.Agent, pgtype.UUID, error) {
	var squadID pgtype.UUID
	if release.ShippedAt.Valid || !release.OwnerID.Valid || !release.OwnerType.Valid {
		return db.Agent{}, squadID, ErrReleaseOwnerUnavailable
	}
	agentID := release.OwnerID
	switch release.OwnerType.String {
	case "agent":
	case "squad":
		squad, err := q.LockReleaseOwnerSquad(ctx, db.LockReleaseOwnerSquadParams{ID: release.OwnerID, WorkspaceID: release.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && squad.ArchivedAt.Valid) {
			return db.Agent{}, squadID, ErrReleaseOwnerUnavailable
		}
		if err != nil {
			return db.Agent{}, squadID, err
		}
		squadID, agentID = squad.ID, squad.LeaderID
	default:
		return db.Agent{}, squadID, ErrReleaseOwnerUnavailable
	}
	agent, err := q.LockReleaseOwnerAgent(ctx, db.LockReleaseOwnerAgentParams{ID: agentID, WorkspaceID: release.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Agent{}, squadID, ErrReleaseOwnerUnavailable
	}
	if err != nil {
		return db.Agent{}, squadID, err
	}
	verdict, err := AgentReadiness(ctx, RuntimeLookup{Queries: q}, agent)
	if err != nil {
		return db.Agent{}, squadID, err
	}
	// Like issue assignment, an offline owner's run waits for its runtime; only
	// an owner nothing will ever claim for is unavailable.
	if verdict.Blocked() {
		return db.Agent{}, squadID, ErrReleaseOwnerUnavailable
	}
	return agent, squadID, nil
}

const SystemRuleReleaseCoordination = "release_coordination"

// CaptureReleaseOwnerChange commits the deliberate trigger with the owner write.
// Subsequent comment and terminal issue facts are captured by database triggers.
func CaptureReleaseOwnerChange(ctx context.Context, q *db.Queries, release db.Release, actorType string, actorID, originator pgtype.UUID) error {
	if !release.OwnerID.Valid || release.ShippedAt.Valid {
		return nil
	}
	_, err := q.GetReleaseCoordinationWakeup(ctx, release.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = q.CreateReleaseCoordinationWakeup(ctx, db.CreateReleaseCoordinationWakeupParams{ID: dbid.NewV7(), WorkspaceID: release.WorkspaceID, ReleaseID: release.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = q.GetReleaseCoordinationWakeup(ctx, release.ID)
		}
	}
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"release_id": util.UUIDToString(release.ID), "actor_type": actorType, "actor_id": util.UUIDToString(actorID),
		"originator_user_id": util.UUIDToString(originator), "owner_type": release.OwnerType.String, "owner_id": util.UUIDToString(release.OwnerID),
	})
	if err != nil {
		return err
	}
	return q.CaptureReleaseWakeupEvent(ctx, db.CaptureReleaseWakeupEventParams{ReleaseID: release.ID,
		EventKey: fmt.Sprintf("release-owner:%s", util.UUIDToString(dbid.NewV7())), EventType: "release.owner_changed", Payload: payload})
}

// ProcessReleaseCoordination is a post-commit nudge of the existing dispatcher.
// Durable receipts also make the ordinary leased tick sufficient after a crash.
func (s *IssueWakeupService) ProcessReleaseCoordination(ctx context.Context, releaseID pgtype.UUID) error {
	rule, err := s.Tasks.Queries.GetReleaseCoordinationWakeup(ctx, releaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.dispatchRelease(ctx, rule)
}
