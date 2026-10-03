package execenv

import (
	"strings"
	"testing"
)

// A project pins a repo to a branch because its work lives on that line
// (MUL-7504). The agent cuts its worktree from there, so the two things the
// BRIEF has to add are which line the work is on, and that delivery goes back
// to the same one.
//
// The second matters most. Nothing in the platform creates pull requests —
// `gh pr create` is the agent's own call, and without --base it targets the
// repository's default branch. A PR opened that way asks to merge into the
// wrong place AND carries every commit the pinned branch has that the default
// lacks.
func TestRepositoriesSectionCarriesPinnedRefAndDeliveryTarget(t *testing.T) {
	t.Parallel()
	ctx := TaskContextForEnv{
		IssueID: "i-1", AgentName: "Eve", AgentID: "eve-1",
		Repos: []RepoContextForEnv{
			{URL: "https://github.com/multica-ai/multica", Ref: "release/2026-09"},
		},
	}
	out := buildMetaSkillContent("claude", ctx)

	if !strings.Contains(out, "https://github.com/multica-ai/multica (starts from `release/2026-09`)") {
		t.Errorf("Repositories section should name the pinned ref beside its repo; got:\n%s", out)
	}
	if !strings.Contains(out, "gh pr create --base") {
		t.Errorf("brief should tell the agent to target the same branch when delivering; got:\n%s", out)
	}
}

// The delivery rule is stated only when something is actually pinned. An
// unpinned workspace repo already starts from its default branch, and
// `gh pr create` already targets that — an unconditional --base instruction
// would be noise in the common case.
func TestRepositoriesSectionOmitsDeliveryRuleWhenNothingIsPinned(t *testing.T) {
	t.Parallel()
	ctx := TaskContextForEnv{
		IssueID: "i-1", AgentName: "Eve", AgentID: "eve-1",
		Repos: []RepoContextForEnv{
			{URL: "https://github.com/multica-ai/multica", Description: "the platform"},
		},
	}
	out := buildMetaSkillContent("claude", ctx)

	if !strings.Contains(out, "https://github.com/multica-ai/multica — the platform") {
		t.Errorf("unpinned repo should still render its description; got:\n%s", out)
	}
	if strings.Contains(out, "starts from") || strings.Contains(out, "gh pr create --base") {
		t.Errorf("no repo is pinned, so the brief must not mention a starting point or --base; got:\n%s", out)
	}
}

// A pin is not necessarily a branch — the field accepts anything git resolves,
// and neither the server nor the daemon can tell a branch from a tag or a
// commit without asking the remote, which the product deliberately does not do.
// A tag has no branch to merge back into, so the delivery rule has to be stated
// conditionally rather than as "always --base whatever is pinned".
func TestDeliveryRuleDoesNotAssumeThePinIsABranch(t *testing.T) {
	t.Parallel()
	ctx := TaskContextForEnv{
		IssueID: "i-1", AgentName: "Eve", AgentID: "eve-1",
		Repos: []RepoContextForEnv{{URL: "https://github.com/o/r", Ref: "v1.4.0"}},
	}
	out := buildMetaSkillContent("claude", ctx)

	if !strings.Contains(out, "tag or a commit rather than a branch") {
		t.Errorf("brief should carve out the tag/commit case; got:\n%s", out)
	}
	if !strings.Contains(out, "confirm the target branch") {
		t.Errorf("brief should tell the agent to confirm rather than guess a base; got:\n%s", out)
	}
}

// The resume warning protects work that began under a starting point the brief
// can no longer show, so it rides on every repo, pinned or not. The retarget
// sentence points at the listed starting point, so it appears only when one is
// listed. Without repos the section is skipped whole.
func TestResumeWarningGating(t *testing.T) {
	t.Parallel()
	const warning = "If this task's worktree or pull request already exists"
	const retarget = "Do not retarget it to a starting point listed above"
	for _, tc := range []struct {
		name                      string
		repos                     []RepoContextForEnv
		wantWarning, wantRetarget bool
	}{
		{"pinned", []RepoContextForEnv{{URL: "https://github.com/o/r", Ref: "release/b"}}, true, true},
		{"starting point cleared", []RepoContextForEnv{{URL: "https://github.com/o/r"}}, true, false},
		{"no repos", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := buildMetaSkillContent("claude", TaskContextForEnv{
				IssueID: "i-1", AgentName: "Eve", AgentID: "eve-1", Repos: tc.repos,
			})
			if got := strings.Contains(out, warning); got != tc.wantWarning {
				t.Errorf("resume warning present = %v, want %v; got:\n%s", got, tc.wantWarning, out)
			}
			if got := strings.Contains(out, retarget); got != tc.wantRetarget {
				t.Errorf("retarget sentence present = %v, want %v; got:\n%s", got, tc.wantRetarget, out)
			}
		})
	}
}

// Every task kind that can change code gets the worktree rule, and no brief
// still sends the agent to `multica repo checkout`, whose branches carry
// Multica ids. Quick-create only files issues, so it gets neither.
func TestWorktreeRuleReachesEveryCodeTaskKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		ctx  TaskContextForEnv
		want bool
	}{
		{"issue", TaskContextForEnv{IssueID: "i-1", AgentName: "Eve", AgentID: "eve-1"}, true},
		{"autopilot run-only", TaskContextForEnv{AutopilotRunID: "r-1", AgentName: "Eve", AgentID: "eve-1"}, true},
		{"chat", TaskContextForEnv{ChatSessionID: "c-1", AgentName: "Eve", AgentID: "eve-1"}, true},
		{"quick-create", TaskContextForEnv{QuickCreatePrompt: "p", AgentName: "Eve", AgentID: "eve-1"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := buildMetaSkillContent("claude", tc.ctx)
			if got := strings.Contains(out, worktreeRule); got != tc.want {
				t.Errorf("worktree rule present = %v, want %v; got:\n%s", got, tc.want, out)
			}
			if strings.Contains(out, "multica repo checkout") {
				t.Errorf("brief must not send the agent to `multica repo checkout`; got:\n%s", out)
			}
		})
	}
}
