package handler

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// A membership loop between squads stays finite and counts each squad once,
// outermost first with the owning squad last (ADR-0034).
func TestClaim_SquadMembershipLoopCountsEachSquadOnce(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach loop", nil)
	parent := fx.squad(t, "Reach Loop Parent", "Parent rules.")
	owner := fx.squad(t, "Reach Loop Owner", "Owner rules.")
	nestSquad(t, parent, owner)
	nestSquad(t, owner, parent)
	squadSkill(t, parent, "reach-loop-parent-skill")
	squadSkill(t, owner, "reach-loop-owner-skill")

	agent := fx.claimFor(t, squadIssue(t, "Reach loop issue", owner, ""), "")

	want := "Agent rules." +
		"\n\n## Squad Instructions (Reach Loop Parent)\n\nParent rules." +
		"\n\n## Squad Instructions (Reach Loop Owner)\n\nOwner rules."
	if agent.Instructions != want {
		t.Fatalf("instructions = %q, want %q", agent.Instructions, want)
	}
	if got := skillNames(agent); !slices.Equal(got, []string{"reach-loop-owner-skill", "reach-loop-parent-skill"}) {
		t.Fatalf("skills = %v, want each squad's skill once", got)
	}
}

// Squads and issues of another workspace never reach a run, even when a
// malformed row links them to this workspace's squad or issue.
func TestClaim_SquadReachStaysInTheRunsWorkspace(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach scope", nil)
	other := dbfx.Workspace(t, "Reach scope other", "reach-scope-"+uuid.NewString())
	foreignLeader := dbfx.Agent(t, "Reach scope foreign leader", "", testutil.Cols{"workspace_id": other})
	foreign := dbfx.Squad(t, "Reach Scope Foreign", foreignLeader, testutil.Cols{
		"workspace_id": other,
		"instructions": "Foreign rules.",
	})
	squadSkill(t, foreign, "reach-scope-foreign-skill")

	owner := fx.squad(t, "Reach Scope Owner", "Owner rules.")
	squadSkill(t, owner, "reach-scope-owner-skill")
	nestSquad(t, foreign, owner)

	viaMembership := fx.claimFor(t, squadIssue(t, "Reach scope owned issue", owner, ""), "")
	if want := "Agent rules.\n\n## Squad Instructions (Reach Scope Owner)\n\nOwner rules."; viaMembership.Instructions != want {
		t.Fatalf("instructions = %q, want %q", viaMembership.Instructions, want)
	}
	if got := skillNames(viaMembership); !slices.Equal(got, []string{"reach-scope-owner-skill"}) {
		t.Fatalf("skills = %v, want only this workspace's squad", got)
	}

	// The issue is assigned to the foreign squad, and its parent lives in
	// the other workspace under that squad: neither may reach the run.
	foreignParent := dbfx.Issue(t, "Reach scope foreign parent", testutil.Cols{
		"workspace_id":  other,
		"assignee_type": "squad",
		"assignee_id":   foreign,
	})
	viaForeignAssignee := fx.claimFor(t, squadIssue(t, "Reach scope foreign assignee", foreign, ""), "")
	viaForeignParent := fx.claimFor(t, squadIssue(t, "Reach scope foreign child", "", foreignParent), "")
	for name, agent := range map[string]*TaskAgentData{"assignee": viaForeignAssignee, "parent": viaForeignParent} {
		if agent.Instructions != "Agent rules." || len(skillNames(agent)) != 0 {
			t.Fatalf("foreign squad reached the run via its %s: %q %v", name, agent.Instructions, skillNames(agent))
		}
	}
}
