package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// TestListSkills_OmitsContent guards the fix for GH multica-ai/multica#2174:
// the workspace skill list endpoint must not ship the SKILL.md `content`
// blob, which used to bloat the payload past CLI timeouts on workspaces with
// many large skills. The detail endpoint still returns content (covered by
// TestGetSkill_IncludesContent below).
func TestListSkills_OmitsContent(t *testing.T) {
	skillID := insertHandlerTestSkill(t, "list-omits-content", strings.Repeat("a", 4096))

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/skills?workspace_id="+testWorkspaceID, nil)
	testHandler.ListSkills(w, req)
	if w.Code != 200 {
		t.Fatalf("ListSkills: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Decode into a generic shape so we can prove the wire format has no
	// `content` field at all — not "content present but empty", which would
	// still leave the bytes on the wire.
	var rows []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("ListSkills: failed to decode body: %v", err)
	}

	var found bool
	for _, row := range rows {
		if row["id"] != skillID {
			continue
		}
		found = true
		if _, ok := row["content"]; ok {
			t.Fatalf("ListSkills: response must not include `content` field, got: %v", row)
		}
		// Other expected list fields should still be present.
		for _, key := range []string{"id", "name", "description", "config", "created_at", "updated_at", "workspace_id"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("ListSkills: missing expected field %q in response: %v", key, row)
			}
		}
	}
	if !found {
		t.Fatalf("ListSkills: inserted skill %s not in response", skillID)
	}
}

// TestGetSkill_IncludesContent confirms the detail endpoint still ships the
// full SKILL.md body — the list-summary change must not regress single-skill
// reads.
func TestGetSkill_IncludesContent(t *testing.T) {
	body := "# detail body\nstill served on /api/skills/{id}"
	skillID := insertHandlerTestSkill(t, "detail-includes-content", body)

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/skills/"+skillID, nil)
	req = withURLParam(req, "id", skillID)
	testHandler.GetSkill(w, req)
	if w.Code != 200 {
		t.Fatalf("GetSkill: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetSkill: failed to decode body: %v", err)
	}
	if got, _ := resp["content"].(string); got != body {
		t.Fatalf("GetSkill: expected content %q, got %q", body, got)
	}
}

// TestListAgentSkills_OmitsContent: same constraint for the agent-scoped
// listing — gpt-boy review of the original fix flagged this as a sister case
// because `multica agent skills list` follows the same shape rules.
func TestListAgentSkills_OmitsContent(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Handler Skill Summary Test", nil)
	skillID := insertHandlerTestSkill(t, "agent-skill-omits-content", strings.Repeat("b", 1024))
	if _, err := testPool.Exec(context.Background(),
		`INSERT INTO agent_skill (agent_id, skill_id) VALUES ($1, $2)`,
		agentID, skillID,
	); err != nil {
		t.Fatalf("attach skill to agent: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/agents/"+agentID+"/skills", nil)
	req = withURLParam(req, "id", agentID)
	testHandler.ListAgentSkills(w, req)
	if w.Code != 200 {
		t.Fatalf("ListAgentSkills: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("ListAgentSkills: failed to decode body: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("ListAgentSkills: expected at least 1 skill")
	}
	for _, row := range rows {
		if _, ok := row["content"]; ok {
			t.Fatalf("ListAgentSkills: response must not include `content` field, got: %v", row)
		}
	}
}

func TestSetAgentSkillEnabledControlsExecutionWithoutRemovingAssignment(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Handler Skill Toggle Test", nil)
	skillID := insertHandlerTestSkill(t, "agent-skill-toggle", "# Toggle me")
	if _, err := testPool.Exec(context.Background(),
		`INSERT INTO agent_skill (agent_id, skill_id) VALUES ($1, $2)`,
		agentID, skillID,
	); err != nil {
		t.Fatalf("attach skill to agent: %v", err)
	}

	setEnabled := func(enabled bool) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := newRequest("PUT", "/api/agents/"+agentID+"/skills/"+skillID+"/enabled", map[string]any{
			"enabled": enabled,
		})
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", agentID)
		rctx.URLParams.Add("skillId", skillID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		testHandler.SetAgentSkillEnabled(w, req)
		return w
	}

	if w := setEnabled(false); w.Code != 200 {
		t.Fatalf("disable skill: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Disabled assignments remain visible to management surfaces.
	rows, err := testHandler.Queries.ListAgentSkillSummaries(context.Background(), parseUUID(agentID))
	if err != nil || len(rows) != 1 || rows[0].Enabled {
		t.Fatalf("disabled assignment not preserved: rows=%+v err=%v", rows, err)
	}
	// Execution only receives enabled skills.
	active, err := testHandler.Queries.ListAgentSkills(context.Background(), db.ListAgentSkillsParams{AgentID: parseUUID(agentID)})
	if err != nil || len(active) != 0 {
		t.Fatalf("disabled skill leaked into execution: skills=%+v err=%v", active, err)
	}

	if w := setEnabled(true); w.Code != 200 {
		t.Fatalf("enable skill: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	active, err = testHandler.Queries.ListAgentSkills(context.Background(), db.ListAgentSkillsParams{AgentID: parseUUID(agentID)})
	if err != nil || len(active) != 1 || active[0].Name == "" {
		t.Fatalf("re-enabled skill missing from execution: skills=%+v err=%v", active, err)
	}
}

func TestSetAgentRuntimeSkillEnabledPersistsScopedException(t *testing.T) {
	runtimeID := createRuntimeLocalSkillTestRuntime(t, testUserID)
	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id
		)
		VALUES ($1, $2, '', 'local', '{}'::jsonb, $3, 'private', 'private', 1, $4)
		RETURNING id
	`, testWorkspaceID, "Runtime Skill Toggle "+t.Name(), runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
	})

	setEnabled := func(enabled bool) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := newRequest("PUT", "/api/agents/"+agentID+"/runtime-skills/enabled", map[string]any{
			"runtime_id": runtimeID,
			"root":       "provider",
			"key":        "review",
			"name":       "Review Helper",
			"enabled":    enabled,
		})
		req = withURLParam(req, "id", agentID)
		testHandler.SetAgentRuntimeSkillEnabled(w, req)
		return w
	}

	if w := setEnabled(false); w.Code != 204 {
		t.Fatalf("disable inherited skill: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	row, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
	if err != nil {
		t.Fatalf("load agent: %v", err)
	}
	exceptions := decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions)
	if len(exceptions) != 1 || exceptions[0].RuntimeID != runtimeID ||
		exceptions[0].Provider != "claude" || exceptions[0].Root != "provider" || exceptions[0].Key != "review" ||
		exceptions[0].Name != "Review Helper" {
		t.Fatalf("unexpected runtime skill exceptions: %+v", exceptions)
	}
	if w := setEnabled(false); w.Code != 204 {
		t.Fatalf("repeat disable inherited skill: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	row, err = testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
	if err != nil || len(decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions)) != 1 {
		t.Fatalf("repeat disable was not idempotent: row=%+v err=%v", row, err)
	}

	if w := setEnabled(true); w.Code != 204 {
		t.Fatalf("enable inherited skill: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	row, err = testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
	if err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	if exceptions = decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions); len(exceptions) != 0 {
		t.Fatalf("re-enabled skill is still an exception: %+v", exceptions)
	}

	// With the section switch off, a skill that is turned on is the exception.
	if _, err := testPool.Exec(context.Background(), `UPDATE agent SET inherit_runtime_skills = false WHERE id = $1`, agentID); err != nil {
		t.Fatalf("turn section off: %v", err)
	}
	if w := setEnabled(true); w.Code != 204 {
		t.Fatalf("pick skill while section off: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	row, err = testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
	if err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	if exceptions = decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions); len(exceptions) != 1 || exceptions[0].Key != "review" {
		t.Fatalf("picked skill is not stored while section off: %+v", exceptions)
	}
	if w := setEnabled(false); w.Code != 204 {
		t.Fatalf("unpick skill while section off: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	row, err = testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
	if err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	if exceptions = decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions); len(exceptions) != 0 {
		t.Fatalf("unpicked skill is still an exception: %+v", exceptions)
	}
}

func TestSetAgentRuntimeSkillEnabledProviderAndPermission(t *testing.T) {
	memberID := createRuntimeLocalSkillTestMember(t, "member")
	for _, tc := range []struct {
		name       string
		provider   string
		mode       string
		root       string
		member     bool
		wantStatus int
	}{
		{name: "omp provider skill", provider: "omp", mode: "local", root: "provider", wantStatus: http.StatusNoContent},
		{name: "omp universal skill", provider: "omp", mode: "local", root: "universal", wantStatus: http.StatusNoContent},
		{name: "codex provider skill", provider: "codex", mode: "local", root: "provider", wantStatus: http.StatusNoContent},
		{name: "claude plugin skill", provider: "claude", mode: "local", root: "plugin", wantStatus: http.StatusNoContent},
		{name: "omp plugin rejected", provider: "omp", mode: "local", root: "plugin", wantStatus: http.StatusBadRequest},
		{name: "codex plugin rejected", provider: "codex", mode: "local", root: "plugin", wantStatus: http.StatusBadRequest},
		{name: "unsupported provider", provider: "pi", mode: "local", root: "provider", wantStatus: http.StatusBadRequest},
		{name: "non local runtime", provider: "omp", mode: "cloud", root: "provider", wantStatus: http.StatusBadRequest},
		{name: "member cannot manage", provider: "omp", mode: "local", root: "provider", member: true, wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtimeID := dbfx.Runtime(t, t.Name(), testutil.Cols{"runtime_mode": tc.mode, "provider": tc.provider})
			agentID := dbfx.Agent(t, t.Name(), runtimeID, testutil.Cols{
				"runtime_mode": tc.mode, "visibility": "workspace", "permission_mode": "public_to",
			})
			if tc.member {
				dbfx.Insert(t, "agent_invocation_target", testutil.Cols{
					"agent_id": agentID, "target_type": "member", "target_id": memberID, "created_by": testUserID,
				})
			}
			userID := testUserID
			if tc.member {
				userID = memberID
			}
			req := newRequestAsUser(userID, "PUT", "/api/agents/"+agentID+"/runtime-skills/enabled", map[string]any{
				"runtime_id": runtimeID,
				"root":       tc.root,
				"key":        "review",
				"name":       "Review",
				"plugin":     "review-tools",
				"enabled":    false,
			})
			req = withURLParam(req, "id", agentID)
			testutil.Call(t, testHandler.SetAgentRuntimeSkillEnabled, req).Want(tc.wantStatus)
			row, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
			if err != nil {
				t.Fatalf("load agent after runtime skill request: %v", err)
			}
			exceptions := decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions)
			if tc.wantStatus == http.StatusNoContent {
				plugin := ""
				if tc.root == "plugin" {
					plugin = "review-tools"
				}
				want := RuntimeSkillException{
					RuntimeID: runtimeID, Provider: tc.provider, Root: tc.root, Key: "review", Name: "Review", Plugin: plugin,
				}
				if len(exceptions) != 1 || exceptions[0] != want {
					t.Fatalf("runtime skill exception = %+v, want %+v", exceptions, want)
				}
			} else if len(exceptions) != 0 {
				t.Fatalf("rejected runtime skill change saved choices: %+v", exceptions)
			}
			if !row.InheritRuntimeSkills {
				t.Fatal("per-skill change modified section inheritance")
			}
		})
	}
}

// TestGetSkill_MalformedUUIDReturns400 guards the handler UUID parsing
// convention (CLAUDE.md → "Backend Handler UUID Parsing Convention"): raw
// `id` URL params on the request boundary must be validated with
// parseUUIDOrBadRequest, not the panic-prone parseUUID. Before the fix
// the malformed input panicked in MustParseUUID and was rescued by the
// chi Recoverer middleware as a 500.
func TestGetSkill_MalformedUUIDReturns400(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/skills/not-a-uuid", nil)
	req = withURLParam(req, "id", "not-a-uuid")
	testHandler.GetSkill(w, req)
	if w.Code != 400 {
		t.Fatalf("GetSkill malformed uuid: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// insertHandlerTestSkill writes a skill row directly via SQL and registers a
// cleanup hook. We bypass the create handler to keep the test focused on the
// list/detail wire shape and to make it easy to inject a large body.
func insertHandlerTestSkill(t *testing.T, namePrefix, content string) string {
	t.Helper()
	name := namePrefix + "-" + t.Name()
	var id string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO skill (workspace_id, name, description, content, config, created_by)
		VALUES ($1, $2, $3, $4, '{}'::jsonb, $5)
		RETURNING id
	`, testWorkspaceID, name, "fixture", content, testUserID).Scan(&id); err != nil {
		t.Fatalf("insert skill: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM skill WHERE id = $1`, id)
	})
	return id
}

func TestListSkills_IncludesAttachedLabelsAndOmitsContent(t *testing.T) {
	labeledID := insertHandlerTestSkill(t, "list-skill-with-label", strings.Repeat("c", 512))
	unlabeledID := insertHandlerTestSkill(t, "list-skill-no-label", strings.Repeat("d", 512))

	wCreate := httptest.NewRecorder()
	testHandler.CreateLabel(wCreate, newRequest("POST", "/api/labels", map[string]any{
		"resource_type": "skill",
		"name":          "list-filter",
		"color":         "#3b82f6",
	}))
	if wCreate.Code != 201 {
		t.Fatalf("CreateLabel: expected 201, got %d: %s", wCreate.Code, wCreate.Body.String())
	}
	var created LabelResponse
	if err := json.NewDecoder(wCreate.Body).Decode(&created); err != nil {
		t.Fatalf("decode created label: %v", err)
	}
	t.Cleanup(func() {
		w := httptest.NewRecorder()
		req := newRequest("DELETE", "/api/labels/"+created.ID, nil)
		req = withURLParam(req, "id", created.ID)
		testHandler.DeleteLabel(w, req)
	})
	if _, err := testPool.Exec(context.Background(),
		`INSERT INTO skill_to_label (skill_id, label_id) VALUES ($1, $2)`,
		labeledID, created.ID,
	); err != nil {
		t.Fatalf("attach skill label: %v", err)
	}

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/skills?workspace_id="+testWorkspaceID, nil)
	testHandler.ListSkills(w, req)
	if w.Code != 200 {
		t.Fatalf("ListSkills: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatalf("ListSkills: failed to decode body: %v", err)
	}

	byID := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		id, _ := row["id"].(string)
		byID[id] = row
	}

	labeled, ok := byID[labeledID]
	if !ok {
		t.Fatalf("ListSkills: labeled skill %s not in response", labeledID)
	}
	if _, hasContent := labeled["content"]; hasContent {
		t.Fatalf("ListSkills: labeled skill must still omit content, got: %v", labeled)
	}
	labels, ok := labeled["labels"].([]any)
	if !ok || len(labels) != 1 {
		t.Fatalf("ListSkills: labeled skill expected 1 label, got: %v", labeled["labels"])
	}
	label, _ := labels[0].(map[string]any)
	if label["id"] != created.ID || label["name"] != created.Name || label["resource_type"] != "skill" {
		t.Fatalf("ListSkills: unexpected attached label: %+v", label)
	}

	unlabeled, ok := byID[unlabeledID]
	if !ok {
		t.Fatalf("ListSkills: unlabeled skill %s not in response", unlabeledID)
	}
	empty, ok := unlabeled["labels"].([]any)
	if !ok || len(empty) != 0 {
		t.Fatalf("ListSkills: unlabeled skill expected labels: [], got: %v", unlabeled["labels"])
	}
}
