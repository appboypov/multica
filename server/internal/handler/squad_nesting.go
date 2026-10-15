package handler

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Sub-squads (ADR-0017): a squad member may be a squad. Stored data never
// holds a loop or a sub-squad that shares its main squad's leader, so every
// reader can trust direct rows. Both checks run under LockSquadNesting, in
// the transaction that writes the membership or the leader.

// squadNestingRefusal returns why child may not become a member of parent,
// or "" when it may. A loop at any depth is named from parent back to parent.
func squadNestingRefusal(ctx context.Context, q *db.Queries, parent, child db.Squad) (string, error) {
	path, err := squadNestingLoop(ctx, q, parent, child)
	if err != nil {
		return "", err
	}
	if path != nil {
		names := make([]string, 0, len(path)+1)
		names = append(names, parent.Name)
		for _, id := range path {
			squad, err := q.GetSquad(ctx, id)
			if err != nil {
				return "", err
			}
			names = append(names, squad.Name)
		}
		return "adding " + child.Name + " would create a loop: " + strings.Join(names, " → "), nil
	}
	// The main leader hands work to a sub-squad by @mentioning it; when both
	// squads share a leader, that mention is the leader's own and is suppressed
	// as a self-trigger, so the hand-off would stall.
	if parent.LeaderID == child.LeaderID {
		return child.Name + " has the same leader as " + parent.Name, nil
	}
	return "", nil
}

// squadNestingLoop returns the shortest path of squads from child down its
// squad members to parent, or nil when parent is not below child; a child
// that is the parent is a one-squad path. The walk is breadth-first and
// visits each squad once: nesting has no depth limit and a squad may sit in
// several squads, so walking every path instead grows exponentially with
// shared sub-squads, all while the add holds the workspace nesting lock.
// Archived squads stay in the walk: their memberships still exist.
func squadNestingLoop(ctx context.Context, q *db.Queries, parent, child db.Squad) ([]pgtype.UUID, error) {
	if child.ID == parent.ID {
		return []pgtype.UUID{child.ID}, nil
	}
	memberships, err := q.ListSquadNestingMemberships(ctx, parent.WorkspaceID)
	if err != nil {
		return nil, err
	}
	subSquads := make(map[pgtype.UUID][]pgtype.UUID)
	for _, m := range memberships {
		subSquads[m.SquadID] = append(subSquads[m.SquadID], m.MemberID)
	}
	// reachedFrom maps each visited squad to the squad above it on its
	// shortest path from child.
	reachedFrom := map[pgtype.UUID]pgtype.UUID{child.ID: {}}
	queue := []pgtype.UUID{child.ID}
	for len(queue) > 0 {
		squad := queue[0]
		queue = queue[1:]
		for _, sub := range subSquads[squad] {
			if _, seen := reachedFrom[sub]; seen {
				continue
			}
			reachedFrom[sub] = squad
			if sub != parent.ID {
				queue = append(queue, sub)
				continue
			}
			path := []pgtype.UUID{sub}
			for at := squad; at != child.ID; at = reachedFrom[at] {
				path = append(path, at)
			}
			path = append(path, child.ID)
			slices.Reverse(path)
			return path, nil
		}
	}
	return nil, nil
}

// squadLeaderNestingRefusal returns why leaderID may not lead squad, or ""
// when it may: it must not lead a squad directly above or below squad.
func squadLeaderNestingRefusal(ctx context.Context, q *db.Queries, squad db.Squad, leaderID pgtype.UUID, leaderName string) (string, error) {
	nested, err := q.ListSquadsNestedWithLeader(ctx, db.ListSquadsNestedWithLeaderParams{
		LeaderID: leaderID,
		SquadID:  squad.ID,
	})
	if err != nil {
		return "", err
	}
	if len(nested) == 0 {
		return "", nil
	}
	other := nested[0]
	squadIsMember, err := q.IsSquadMember(ctx, db.IsSquadMemberParams{
		SquadID: other.ID, MemberType: "squad", MemberID: squad.ID,
	})
	if err != nil {
		return "", err
	}
	reason := other.Name + ", a member of " + squad.Name + ", is led by " + leaderName
	if squadIsMember {
		reason = squad.Name + " sits in " + other.Name + ", which " + leaderName + " leads"
	}
	return reason + "; a sub-squad cannot share its leader with its main squad", nil
}
