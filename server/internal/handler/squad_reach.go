package handler

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// loadReachingSquads returns the squads that reach a task's run (ADR-0034),
// outermost first and the squad owning the issue last. Runs without an issue,
// such as quick-create and release runs, are reached by no squad.
func (h *Handler) loadReachingSquads(ctx context.Context, issueID, workspaceID pgtype.UUID) ([]db.Squad, error) {
	if !issueID.Valid {
		return nil, nil
	}
	return h.Queries.ListIssueContributingSquads(ctx, db.ListIssueContributingSquadsParams{
		IssueID:     issueID,
		WorkspaceID: workspaceID,
	})
}

func squadIDsOf(squads []db.Squad) []pgtype.UUID {
	if len(squads) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, len(squads))
	for i, squad := range squads {
		ids[i] = squad.ID
	}
	return ids
}

func squadReaches(squads []db.Squad, squadID pgtype.UUID) bool {
	for _, squad := range squads {
		if squad.ID == squadID {
			return true
		}
	}
	return false
}

// appendSquadInstructions appends each reaching squad's user instructions to
// the agent's, in the order given: outermost squad first, so the most specific
// rule reads last.
func appendSquadInstructions(instructions string, squads []db.Squad) string {
	var sb strings.Builder
	sb.WriteString(instructions)
	for _, squad := range squads {
		section := squadInstructionsSection(squad)
		if section == "" {
			continue
		}
		if strings.TrimSpace(sb.String()) != "" {
			sb.WriteString("\n\n")
		}
		sb.WriteString(section)
	}
	return sb.String()
}

// squadInstructionsSection renders a squad's user instructions under their
// heading, or "" when it has none so no heading dangles.
func squadInstructionsSection(squad db.Squad) string {
	trimmed := strings.TrimSpace(squad.Instructions)
	if trimmed == "" {
		return ""
	}
	return "## Squad Instructions (" + squad.Name + ")\n\n" + trimmed
}

// runtimeSkillPolicy is one runtime skill section switch with its exceptions,
// already scoped to the claimed runtime and provider.
type runtimeSkillPolicy struct {
	inherit    bool
	exceptions []RuntimeSkillException
}

// combineRuntimeSkillPolicies returns the policy that allows a runtime skill
// when any of the given policies allows it (ADR-0034), in the section switch
// plus exceptions form the daemon already expands. A policy allows every
// skill except its exceptions while its switch is on, and only its exceptions
// while it is off. So the union is on as soon as one policy is on, keeping
// out only the skills every on policy keeps out and no off policy takes;
// otherwise it is off, taking every skill any policy takes.
func combineRuntimeSkillPolicies(policies []runtimeSkillPolicy) runtimeSkillPolicy {
	identity := func(skill RuntimeSkillException) string {
		return skill.Root + "\x00" + skill.Key + "\x00" + skill.Plugin
	}
	var on []runtimeSkillPolicy
	taken := make(map[string]struct{})
	for _, policy := range policies {
		if policy.inherit {
			on = append(on, policy)
			continue
		}
		for _, skill := range policy.exceptions {
			taken[identity(skill)] = struct{}{}
		}
	}
	combined := runtimeSkillPolicy{exceptions: []RuntimeSkillException{}}
	seen := make(map[string]struct{})
	if len(on) == 0 {
		for _, policy := range policies {
			for _, skill := range policy.exceptions {
				id := identity(skill)
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
				combined.exceptions = append(combined.exceptions, skill)
			}
		}
		return combined
	}
	combined.inherit = true
	keptOut := make([]map[string]struct{}, len(on))
	for i, policy := range on {
		keptOut[i] = make(map[string]struct{}, len(policy.exceptions))
		for _, skill := range policy.exceptions {
			keptOut[i][identity(skill)] = struct{}{}
		}
	}
	for _, skill := range on[0].exceptions {
		id := identity(skill)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := taken[id]; ok {
			continue
		}
		everyOn := true
		for _, set := range keptOut[1:] {
			if _, ok := set[id]; !ok {
				everyOn = false
				break
			}
		}
		if everyOn {
			combined.exceptions = append(combined.exceptions, skill)
		}
	}
	return combined
}

// reachingRuntimeSkillPolicy combines the agent's runtime skill policy with
// those of the squads reaching the run, on the claimed runtime and provider.
func reachingRuntimeSkillPolicy(agent db.Agent, squads []db.Squad, runtimeID, provider string) runtimeSkillPolicy {
	agentPolicy := runtimeSkillPolicy{
		inherit:    agent.InheritRuntimeSkills,
		exceptions: runtimeSkillExceptionsFor(agent.RuntimeSkillExceptions, runtimeID, provider),
	}
	if len(squads) == 0 {
		return agentPolicy
	}
	policies := make([]runtimeSkillPolicy, 0, len(squads)+1)
	policies = append(policies, agentPolicy)
	for _, squad := range squads {
		policies = append(policies, runtimeSkillPolicy{
			inherit:    squad.InheritRuntimeSkills,
			exceptions: runtimeSkillExceptionsFor(squad.RuntimeSkillExceptions, runtimeID, provider),
		})
	}
	return combineRuntimeSkillPolicies(policies)
}
