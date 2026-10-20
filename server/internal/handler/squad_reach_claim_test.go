package handler

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// squadReachFixture is one claiming agent on its own omp runtime, so every
// claim in a test picks exactly the task that test queued.
type squadReachFixture struct {
	runtimeID string
	agentID   string
	leaderID  string
}

func newSquadReachFixture(t *testing.T, name string, agentCols testutil.Cols) squadReachFixture {
	t.Helper()
	runtimeID := dbfx.Runtime(t, name+" runtime", testutil.Cols{"provider": "omp"})
	cols := testutil.Cols{"instructions": "Agent rules."}
	for k, v := range agentCols {
		cols[k] = v
	}
	return squadReachFixture{
		runtimeID: runtimeID,
		agentID:   dbfx.Agent(t, name+" worker", runtimeID, cols),
		leaderID:  dbfx.Agent(t, name+" leader", ""),
	}
}

// squad creates a squad led by the fixture's leader.
func (fx squadReachFixture) squad(t *testing.T, name, instructions string, over ...testutil.Cols) string {
	t.Helper()
	cols := testutil.Cols{"instructions": instructions}
	for _, o := range over {
		for k, v := range o {
			cols[k] = v
		}
	}
	return dbfx.Squad(t, name, fx.leaderID, cols)
}

// nest makes child a member of parent.
func nestSquad(t *testing.T, parentID, childID string) {
	t.Helper()
	dbfx.SquadMember(t, parentID, "squad", childID)
}

// squadSkill creates a workspace skill with one file and assigns it to squadID.
func squadSkill(t *testing.T, squadID, name string) string {
	t.Helper()
	skillID := dbfx.Insert(t, "skill", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"name":         name,
		"description":  name + " description",
		"content":      name + " content",
		"config":       testutil.Raw("'{}'::jsonb"),
		"created_by":   testUserID,
	})
	dbfx.Insert(t, "skill_file", testutil.Cols{"skill_id": skillID, "path": "rules.md", "content": name + " rules"})
	dbfx.InsertNoID(t, "squad_skill", testutil.Cols{"squad_id": squadID, "skill_id": skillID},
		"squad_id = $1 AND skill_id = $2", squadID, skillID)
	return skillID
}

func squadIssue(t *testing.T, title, squadID, parentID string) string {
	t.Helper()
	cols := testutil.Cols{}
	if squadID != "" {
		cols["assignee_type"] = "squad"
		cols["assignee_id"] = squadID
	}
	if parentID != "" {
		cols["parent_issue_id"] = parentID
	}
	return dbfx.Issue(t, title, cols)
}

// claimFor queues a task for the fixture's agent on issueID and claims it.
// The claimed task is then cancelled, so a test may claim the same issue again
// without tripping the one-pending-task-per-issue rule.
func (fx squadReachFixture) claimFor(t *testing.T, issueID, capabilities string) *TaskAgentData {
	t.Helper()
	taskID := dbfx.Task(t, fx.agentID, testutil.Cols{"runtime_id": fx.runtimeID, "issue_id": issueID})
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+fx.runtimeID+"/tasks/claim", nil, testWorkspaceID, "squad-reach")
	if capabilities != "" {
		req.Header.Set("X-Client-Capabilities", capabilities)
	}
	req = withURLParam(req, "runtimeId", fx.runtimeID)
	var resp struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&resp)
	if resp.Task == nil || resp.Task.Agent == nil {
		t.Fatal("claim delivered no task agent")
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'cancelled' WHERE id = $1`, taskID)
	return resp.Task.Agent
}

// skillNames lists the workspace skills a claim carries, on either skill
// path; the built-ins every run gets are left out.
func skillNames(agent *TaskAgentData) []string {
	names := []string{}
	for _, s := range agent.Skills {
		if s.ID != "" {
			names = append(names, s.Name)
		}
	}
	for _, s := range agent.SkillRefs {
		if s.Source == "workspace" {
			names = append(names, s.Name)
		}
	}
	slices.Sort(names)
	return names
}

func TestClaim_IssueAssignedToSquadGivesItsSkillsAndInstructions(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach direct", nil)
	squadID := fx.squad(t, "Reach Direct Squad", "Follow the direct squad.")
	squadSkill(t, squadID, "reach-direct-skill")
	issueID := squadIssue(t, "Reach direct issue", squadID, "")

	agent := fx.claimFor(t, issueID, "")

	want := "Agent rules.\n\n## Squad Instructions (Reach Direct Squad)\n\nFollow the direct squad."
	if agent.Instructions != want {
		t.Fatalf("instructions = %q, want %q", agent.Instructions, want)
	}
	if !slices.Contains(skillNames(agent), "reach-direct-skill") {
		t.Fatalf("squad skill missing from claim: %v", skillNames(agent))
	}
}

func TestClaim_SubIssueWithoutSquadGetsClosestAncestorSquad(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach ancestor", nil)
	squadID := fx.squad(t, "Reach Ancestor Squad", "Ancestor rules.")
	squadSkill(t, squadID, "reach-ancestor-skill")
	top := squadIssue(t, "Reach ancestor top", squadID, "")
	middle := squadIssue(t, "Reach ancestor middle", "", top)
	leaf := squadIssue(t, "Reach ancestor leaf", "", middle)

	agent := fx.claimFor(t, leaf, "")

	if want := "Agent rules.\n\n## Squad Instructions (Reach Ancestor Squad)\n\nAncestor rules."; agent.Instructions != want {
		t.Fatalf("instructions = %q, want %q", agent.Instructions, want)
	}
	if !slices.Contains(skillNames(agent), "reach-ancestor-skill") {
		t.Fatalf("ancestor squad skill missing: %v", skillNames(agent))
	}
}

func TestClaim_NearerSquadWinsOverFartherOne(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach nearer", nil)
	far := fx.squad(t, "Reach Far Squad", "Far rules.")
	near := fx.squad(t, "Reach Near Squad", "Near rules.")
	squadSkill(t, far, "reach-far-skill")
	squadSkill(t, near, "reach-near-skill")
	top := squadIssue(t, "Reach nearer top", far, "")
	middle := squadIssue(t, "Reach nearer middle", near, top)
	leaf := squadIssue(t, "Reach nearer leaf", "", middle)

	agent := fx.claimFor(t, leaf, "")

	if want := "Agent rules.\n\n## Squad Instructions (Reach Near Squad)\n\nNear rules."; agent.Instructions != want {
		t.Fatalf("instructions = %q, want %q", agent.Instructions, want)
	}
	if got := skillNames(agent); !slices.Equal(got, []string{"reach-near-skill"}) {
		t.Fatalf("skills = %v, want only the nearer squad's", got)
	}
}

// Diamond: Owner sits in Left and Right, which both sit in Top. Top counts
// once, and instructions run outermost first with the owning squad last.
func TestClaim_ParentSquadsContributeAndDiamondCountsEachOnce(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach diamond", nil)
	top := fx.squad(t, "Reach Top", "Top rules.")
	left := fx.squad(t, "Reach Left", "Left rules.")
	right := fx.squad(t, "Reach Right", "Right rules.")
	owner := fx.squad(t, "Reach Owner", "Owner rules.")
	nestSquad(t, top, left)
	nestSquad(t, top, right)
	nestSquad(t, left, owner)
	nestSquad(t, right, owner)
	squadSkill(t, top, "reach-top-skill")
	squadSkill(t, left, "reach-left-skill")
	squadSkill(t, owner, "reach-owner-skill")
	issueID := squadIssue(t, "Reach diamond issue", owner, "")

	agent := fx.claimFor(t, issueID, "")

	want := "Agent rules." +
		"\n\n## Squad Instructions (Reach Top)\n\nTop rules." +
		"\n\n## Squad Instructions (Reach Left)\n\nLeft rules." +
		"\n\n## Squad Instructions (Reach Right)\n\nRight rules." +
		"\n\n## Squad Instructions (Reach Owner)\n\nOwner rules."
	if agent.Instructions != want {
		t.Fatalf("instructions = %q, want %q", agent.Instructions, want)
	}
	if got := skillNames(agent); !slices.Equal(got, []string{"reach-left-skill", "reach-owner-skill", "reach-top-skill"}) {
		t.Fatalf("skills = %v, want each squad's skill once", got)
	}
}

// ompSkills names provider-root omp runtime skills on runtimeID.
func ompSkills(runtimeID string, keys ...string) []RuntimeSkillException {
	skills := make([]RuntimeSkillException, 0, len(keys))
	for _, key := range keys {
		skills = append(skills, RuntimeSkillException{RuntimeID: runtimeID, Provider: "omp", Root: "provider", Key: key, Name: key})
	}
	return skills
}

// exceptionsCol renders runtime skill exceptions as a jsonb column value.
func exceptionsCol(t *testing.T, skills ...[]RuntimeSkillException) testutil.Raw {
	t.Helper()
	raw, err := json.Marshal(slices.Concat(skills...))
	if err != nil {
		t.Fatal(err)
	}
	return testutil.Raw("'" + string(raw) + "'::jsonb")
}

// setAgentRuntimeSkills stores the worker's runtime skill policy.
func (fx squadReachFixture) setAgentRuntimeSkills(t *testing.T, inherit bool, skills ...[]RuntimeSkillException) {
	t.Helper()
	raw, err := json.Marshal(slices.Concat(skills...))
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE agent SET inherit_runtime_skills = $1, runtime_skill_exceptions = $2::jsonb WHERE id = $3`, inherit, string(raw), fx.agentID)
}

func exceptionKeys(agent *TaskAgentData) []string {
	keys := []string{}
	for _, skill := range agent.RuntimeSkillExceptions {
		keys = append(keys, skill.Key)
	}
	slices.Sort(keys)
	return keys
}

type runtimeSkillPolicyCase struct {
	inherit bool
	keys    []string
}

// A runtime skill is allowed when the agent's policy or any reaching squad's
// policy allows it on the claimed runtime; the claim carries that union as a
// section switch plus exceptions.
func TestClaim_SkillAllowedByAgentOrSquadIsAllowed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		agent  runtimeSkillPolicyCase
		squads []runtimeSkillPolicyCase
		want   runtimeSkillPolicyCase
	}{
		{
			name:   "both off take what either picks",
			agent:  runtimeSkillPolicyCase{false, []string{"alpha"}},
			squads: []runtimeSkillPolicyCase{{false, []string{"beta", "alpha"}}},
			want:   runtimeSkillPolicyCase{false, []string{"alpha", "beta"}},
		},
		{
			name:   "on agent keeps out only what no squad takes",
			agent:  runtimeSkillPolicyCase{true, []string{"alpha", "beta"}},
			squads: []runtimeSkillPolicyCase{{false, []string{"alpha"}}},
			want:   runtimeSkillPolicyCase{true, []string{"beta"}},
		},
		{
			name:   "on squads keep out only what every on policy keeps out",
			agent:  runtimeSkillPolicyCase{false, []string{"alpha"}},
			squads: []runtimeSkillPolicyCase{{true, []string{"beta", "gamma", "alpha"}}, {true, []string{"beta", "alpha"}}},
			want:   runtimeSkillPolicyCase{true, []string{"beta"}},
		},
		{
			name:   "new squad gives nothing",
			agent:  runtimeSkillPolicyCase{true, []string{"alpha"}},
			squads: []runtimeSkillPolicyCase{{false, nil}},
			want:   runtimeSkillPolicyCase{true, []string{"alpha"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newSquadReachFixture(t, "Reach policy", nil)
			// Exceptions stored for another runtime never count on this one.
			otherRuntime := dbfx.Runtime(t, "Reach policy other runtime", testutil.Cols{"provider": "omp"})
			fx.setAgentRuntimeSkills(t, tc.agent.inherit, ompSkills(fx.runtimeID, tc.agent.keys...), ompSkills(otherRuntime, "other-agent"))
			// The first squad owns the issue; each later one holds the previous.
			var owner, held string
			for i, policy := range tc.squads {
				squadID := fx.squad(t, "Reach Policy Squad "+string(rune('A'+i)), "", testutil.Cols{
					"inherit_runtime_skills":   policy.inherit,
					"runtime_skill_exceptions": exceptionsCol(t, ompSkills(fx.runtimeID, policy.keys...), ompSkills(otherRuntime, "other-squad")),
				})
				if held == "" {
					owner = squadID
				} else {
					nestSquad(t, squadID, held)
				}
				held = squadID
			}
			issueID := squadIssue(t, "Reach policy issue", owner, "")

			agent := fx.claimFor(t, issueID, "")

			if agent.InheritRuntimeSkills == nil || *agent.InheritRuntimeSkills != tc.want.inherit {
				t.Fatalf("inherit = %v, want %v", agent.InheritRuntimeSkills, tc.want.inherit)
			}
			want := slices.Clone(tc.want.keys)
			slices.Sort(want)
			if got := exceptionKeys(agent); !slices.Equal(got, want) {
				t.Fatalf("exceptions = %v, want %v", got, want)
			}
			for _, skill := range agent.RuntimeSkillExceptions {
				if skill.RuntimeID != fx.runtimeID || skill.Provider != "omp" {
					t.Fatalf("exception outside the claimed runtime: %+v", skill)
				}
			}
		})
	}
}

func TestClaim_NoSquadOnOrAboveIssueGivesAgentOnlyResult(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach none", nil)
	fx.setAgentRuntimeSkills(t, false, ompSkills(fx.runtimeID, "alpha"))
	// A squad that holds nothing on this issue tree must not leak in.
	squadSkill(t, fx.squad(t, "Reach Unrelated Squad", "Unrelated rules."), "reach-unrelated-skill")
	// A malformed parent cycle above the issue must stay finite.
	a := squadIssue(t, "Reach none a", "", "")
	b := squadIssue(t, "Reach none b", "", a)
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id = $1 WHERE id = $2`, b, a)
	leaf := squadIssue(t, "Reach none leaf", "", b)

	agent := fx.claimFor(t, leaf, "")

	if agent.Instructions != "Agent rules." {
		t.Fatalf("instructions = %q, want the agent's alone", agent.Instructions)
	}
	if got := skillNames(agent); len(got) != 0 {
		t.Fatalf("skills = %v, want none", got)
	}
	if agent.InheritRuntimeSkills == nil || *agent.InheritRuntimeSkills || !slices.Equal(exceptionKeys(agent), []string{"alpha"}) {
		t.Fatalf("runtime skill policy = %v %v, want the agent's own", agent.InheritRuntimeSkills, exceptionKeys(agent))
	}
}

func TestClaim_WorkerGetsSquadInstructionsButNotLeaderProtocolOrRoster(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach worker", nil)
	squadID := fx.squad(t, "Reach Worker Squad", "Worker rules.")
	dbfx.SquadMember(t, squadID, "agent", fx.agentID)
	issueID := squadIssue(t, "Reach worker issue", squadID, "")

	agent := fx.claimFor(t, issueID, "")

	if !strings.Contains(agent.Instructions, "## Squad Instructions (Reach Worker Squad)\n\nWorker rules.") {
		t.Fatalf("worker missing squad instructions:\n%s", agent.Instructions)
	}
	for _, leaderOnly := range []string{"Squad Operating Protocol", "Squad Roster"} {
		if strings.Contains(agent.Instructions, leaderOnly) {
			t.Fatalf("worker received %q:\n%s", leaderOnly, agent.Instructions)
		}
	}
}

func TestClaim_LeaderSeesUserInstructionsOnce(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach leader", nil)
	parent := fx.squad(t, "Reach Leader Parent", "Parent rules.")
	squadID := dbfx.Squad(t, "Reach Leader Squad", fx.agentID, testutil.Cols{"instructions": "Leader squad rules."})
	nestSquad(t, parent, squadID)
	issueID := squadIssue(t, "Reach leader issue", squadID, "")
	dbfx.Task(t, fx.agentID, testutil.Cols{
		"runtime_id": fx.runtimeID, "issue_id": issueID, "is_leader_task": true, "squad_id": squadID,
	})

	_, instructions, isLeader, raw := claimAgentInstructionsForTest(t, fx.runtimeID)

	if !isLeader {
		t.Fatalf("claim was not a leader task: %s", raw)
	}
	for heading, want := range map[string]int{
		"## Squad Instructions (Reach Leader Squad)":  1,
		"## Squad Instructions (Reach Leader Parent)": 1,
		"## Squad Operating Protocol":                 1,
		"## Squad Roster":                             1,
	} {
		if got := strings.Count(instructions, heading); got != want {
			t.Fatalf("%q appears %d times, want %d:\n%s", heading, got, want, instructions)
		}
	}
	if !strings.HasPrefix(instructions, "Agent rules.\n\n## Squad Instructions (Reach Leader Parent)\n\nParent rules.\n\n## Squad Instructions (Reach Leader Squad)\n\nLeader squad rules.\n\n") {
		t.Fatalf("user instructions must follow the agent's, outermost first, before the briefing:\n%s", instructions)
	}
}

func TestClaim_ArchivedSquadsDoNotContribute(t *testing.T) {
	fx := newSquadReachFixture(t, "Reach archived", nil)
	archivedParent := fx.squad(t, "Reach Archived Parent", "Archived parent rules.", testutil.Cols{"archived_at": testutil.Raw("now()")})
	live := fx.squad(t, "Reach Live Squad", "Live rules.")
	nestSquad(t, archivedParent, live)
	squadSkill(t, archivedParent, "reach-archived-parent-skill")
	squadSkill(t, live, "reach-live-skill")
	archivedOwner := fx.squad(t, "Reach Archived Owner", "Archived owner rules.", testutil.Cols{"archived_at": testutil.Raw("now()")})
	squadSkill(t, archivedOwner, "reach-archived-owner-skill")

	viaParent := fx.claimFor(t, squadIssue(t, "Reach archived parent issue", live, ""), "")
	if want := "Agent rules.\n\n## Squad Instructions (Reach Live Squad)\n\nLive rules."; viaParent.Instructions != want {
		t.Fatalf("instructions = %q, want %q", viaParent.Instructions, want)
	}
	if got := skillNames(viaParent); !slices.Equal(got, []string{"reach-live-skill"}) {
		t.Fatalf("skills = %v, want only the live squad's", got)
	}

	owned := fx.claimFor(t, squadIssue(t, "Reach archived owner issue", archivedOwner, ""), "")
	if owned.Instructions != "Agent rules." || len(skillNames(owned)) != 0 {
		t.Fatalf("archived owning squad contributed: %q %v", owned.Instructions, skillNames(owned))
	}
}

// newSquadReachScenario is one issue reached by a squad with a parent squad,
// each carrying a workspace skill and a runtime skill policy.
func newSquadReachScenario(t *testing.T, name string) (squadReachFixture, string) {
	t.Helper()
	fx := newSquadReachFixture(t, name, testutil.Cols{"inherit_runtime_skills": false})
	parent := fx.squad(t, name+" Parent", "Parent rules.", testutil.Cols{
		"inherit_runtime_skills":   true,
		"runtime_skill_exceptions": exceptionsCol(t, ompSkills(fx.runtimeID, "beta")),
	})
	owner := fx.squad(t, name+" Owner", "Owner rules.")
	nestSquad(t, parent, owner)
	squadSkill(t, parent, strings.ToLower(strings.ReplaceAll(name, " ", "-"))+"-parent-skill")
	squadSkill(t, owner, strings.ToLower(strings.ReplaceAll(name, " ", "-"))+"-owner-skill")
	return fx, squadIssue(t, name+" issue", owner, "")
}

func TestClaim_FullContentAndReferenceClaimsAgree(t *testing.T) {
	fx, issueID := newSquadReachScenario(t, "Reach agree")

	full := fx.claimFor(t, issueID, "")
	refs := fx.claimFor(t, issueID, protocol.DaemonCapabilitySkillBundlesV1)

	if len(full.Skills) == 0 || len(full.SkillRefs) != 0 || len(refs.SkillRefs) == 0 || len(refs.Skills) != 0 {
		t.Fatalf("claims did not take their skill paths: full=%d/%d refs=%d/%d", len(full.Skills), len(full.SkillRefs), len(refs.Skills), len(refs.SkillRefs))
	}
	if want := []string{"reach-agree-owner-skill", "reach-agree-parent-skill"}; !slices.Equal(skillNames(full), want) || !slices.Equal(skillNames(refs), want) {
		t.Fatalf("workspace skills: full=%v refs=%v, want %v", skillNames(full), skillNames(refs), want)
	}
	if full.Instructions != refs.Instructions {
		t.Fatalf("instructions differ:\nfull: %q\nrefs: %q", full.Instructions, refs.Instructions)
	}
	if *full.InheritRuntimeSkills != *refs.InheritRuntimeSkills || !slices.Equal(exceptionKeys(full), exceptionKeys(refs)) {
		t.Fatalf("runtime skill policy differs: full=%v %v refs=%v %v", *full.InheritRuntimeSkills, exceptionKeys(full), *refs.InheritRuntimeSkills, exceptionKeys(refs))
	}
}

func TestClaim_BatchClaimGivesTheSameSquadReach(t *testing.T) {
	fx, issueID := newSquadReachScenario(t, "Reach batch")
	single := fx.claimFor(t, issueID, "")

	dbfx.Task(t, fx.agentID, testutil.Cols{"runtime_id": fx.runtimeID, "issue_id": issueID})
	var batch struct {
		Tasks []AgentTaskResponse `json:"tasks"`
	}
	if w := postBatchClaim(t, testWorkspaceID, []string{fx.runtimeID}, 1); w.Code != http.StatusOK {
		t.Fatalf("batch claim: %d %s", w.Code, w.Body.String())
	} else if err := json.Unmarshal(w.Body.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Tasks) != 1 || batch.Tasks[0].Agent == nil {
		t.Fatalf("batch claim delivered %d tasks", len(batch.Tasks))
	}
	got := batch.Tasks[0].Agent

	if got.Instructions != single.Instructions {
		t.Fatalf("instructions differ:\nbatch:  %q\nsingle: %q", got.Instructions, single.Instructions)
	}
	if !slices.Equal(skillNames(got), skillNames(single)) {
		t.Fatalf("skills differ: batch=%v single=%v", skillNames(got), skillNames(single))
	}
	if *got.InheritRuntimeSkills != true || !slices.Equal(exceptionKeys(got), []string{"beta"}) ||
		*single.InheritRuntimeSkills != true || !slices.Equal(exceptionKeys(single), []string{"beta"}) {
		t.Fatalf("runtime skill policy: batch=%v %v single=%v %v, want on except beta",
			*got.InheritRuntimeSkills, exceptionKeys(got), *single.InheritRuntimeSkills, exceptionKeys(single))
	}
}

func TestResolveTaskSkillBundles_ResolvesSquadDerivedSkills(t *testing.T) {
	fx, issueID := newSquadReachScenario(t, "Reach resolve")
	outsider := squadSkill(t, fx.squad(t, "Reach Resolve Outsider", ""), "reach-resolve-outsider-skill")
	taskID := dbfx.Task(t, fx.agentID, testutil.Cols{"runtime_id": fx.runtimeID, "issue_id": issueID})
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+fx.runtimeID+"/tasks/claim", nil, testWorkspaceID, "squad-reach")
	req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilitySkillBundlesV1)
	req = withURLParam(req, "runtimeId", fx.runtimeID)
	var claim struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&claim)
	if claim.Task == nil || claim.Task.ID != taskID {
		t.Fatalf("claimed %+v, want task %s", claim.Task, taskID)
	}

	resolve := func(ref resolveSkillBundleRef) *testutil.Response {
		body := resolveSkillBundlesRequest{Skills: []resolveSkillBundleRef{ref}}
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+fx.runtimeID+"/tasks/"+taskID+"/skill-bundles/resolve", body, testWorkspaceID, "squad-reach")
		return testutil.Call(t, testHandler.ResolveTaskSkillBundles, withURLParams(req, "runtimeId", fx.runtimeID, "taskId", taskID))
	}
	for _, name := range []string{"reach-resolve-owner-skill", "reach-resolve-parent-skill"} {
		var ref service.AgentSkillRefData
		for _, candidate := range claim.Task.Agent.SkillRefs {
			if candidate.Name == name {
				ref = candidate
			}
		}
		if ref.ID == "" {
			t.Fatalf("claim has no ref for %s: %+v", name, claim.Task.Agent.SkillRefs)
		}
		var resp struct {
			Bundles []service.AgentSkillData `json:"bundles"`
		}
		resolve(resolveSkillBundleRef{ID: ref.ID, Source: ref.Source, Hash: ref.Hash}).Want(http.StatusOK).JSON(&resp)
		if len(resp.Bundles) != 1 || resp.Bundles[0].Content != name+" content" || len(resp.Bundles[0].Files) != 1 || resp.Bundles[0].Files[0].Content != name+" rules" {
			t.Fatalf("resolved %s = %+v", name, resp.Bundles)
		}
	}
	// A squad that does not reach the run authorizes nothing.
	resolve(resolveSkillBundleRef{ID: outsider, Source: "workspace", Hash: "sha256:any"}).Want(http.StatusNotFound)
}
