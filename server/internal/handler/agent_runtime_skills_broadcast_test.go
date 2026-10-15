package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// TestSetAgentRuntimeSkillEnabledBroadcastsAgentStatus guards the realtime
// invalidation path for runtime-skill toggles (Howard review on MUL-5101):
// persisting a disabled_runtime_skills override must publish an "agent:status"
// event so every other open web/desktop/mobile client invalidates
// workspaceKeys.agents and drops its stale toggle state — mirroring the
// workspace-skill toggle in writeUpdatedAgentSkills. Before the fix the handler
// committed and returned 204 without broadcasting, so only the initiating tab
// refreshed while other clients kept showing the old state.
func TestSetAgentRuntimeSkillEnabledBroadcastsAgentStatus(t *testing.T) {
	runtimeID := createRuntimeLocalSkillTestRuntime(t, testUserID)
	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id
		)
		VALUES ($1, $2, '', 'local', '{}'::jsonb, $3, 'private', 'private', 1, $4)
		RETURNING id
	`, testWorkspaceID, "Runtime Skill Broadcast "+t.Name(), runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
	})

	// The bus is synchronous, so counts observed right after each request are
	// deterministic; the mutex only guards against a future async listener.
	var mu sync.Mutex
	var broadcasts []string
	testHandler.Bus.Subscribe(protocol.EventAgentStatus, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		agent, ok := payload["agent"].(AgentResponse)
		if !ok {
			return
		}
		mu.Lock()
		broadcasts = append(broadcasts, agent.ID)
		mu.Unlock()
	})

	countFor := func(id string) int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, a := range broadcasts {
			if a == id {
				n++
			}
		}
		return n
	}

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
	if n := countFor(agentID); n != 1 {
		t.Fatalf("disabling a runtime skill must broadcast exactly one agent:status for %s, got %d", agentID, n)
	}

	if w := setEnabled(true); w.Code != 204 {
		t.Fatalf("enable inherited skill: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if n := countFor(agentID); n != 2 {
		t.Fatalf("re-enabling a runtime skill must broadcast a second agent:status for %s, got %d", agentID, n)
	}
}

func TestAgentInheritRuntimeSkillsInResponseAndClaim(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		name := "default on"
		if !enabled {
			name = "saved off"
		}
		t.Run(name, func(t *testing.T) {
			runtimeID := dbfx.Runtime(t, t.Name(), testutil.Cols{
				"runtime_mode": "local",
				"provider":     "omp",
			})
			agentID := dbfx.Agent(t, t.Name(), runtimeID, testutil.Cols{"runtime_mode": "local"})
			if !enabled {
				if _, err := testHandler.Queries.UpdateAgentInheritRuntimeSkills(context.Background(), db.UpdateAgentInheritRuntimeSkillsParams{
					ID:                   parseUUID(agentID),
					InheritRuntimeSkills: false,
				}); err != nil {
					t.Fatalf("turn inheritance off: %v", err)
				}
			}

			var response struct {
				InheritRuntimeSkills *bool `json:"inherit_runtime_skills"`
			}
			req := withURLParam(newRequest("GET", "/api/agents/"+agentID, nil), "id", agentID)
			testutil.Call(t, testHandler.GetAgent, req).Want(http.StatusOK).JSON(&response)
			if response.InheritRuntimeSkills == nil || *response.InheritRuntimeSkills != enabled {
				t.Fatalf("agent inheritance = %v, want present and %t", response.InheritRuntimeSkills, enabled)
			}

			issueID := dbfx.Issue(t, t.Name())
			taskID := dbfx.Task(t, agentID, testutil.Cols{
				"runtime_id":    runtimeID,
				"issue_id":      issueID,
				"status":        "dispatched",
				"dispatched_at": testutil.Raw("now() - interval '120 seconds'"),
			})
			var claim struct {
				Task *struct {
					ID    string         `json:"id"`
					Agent *TaskAgentData `json:"agent"`
				} `json:"task"`
			}
			req = newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil,
				testWorkspaceID, "runtime-skill-inheritance")
			req = withURLParam(req, "runtimeId", runtimeID)
			testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&claim)
			if claim.Task == nil || claim.Task.ID != taskID || claim.Task.Agent == nil {
				t.Fatalf("expected task %s with agent data, got %+v", taskID, claim.Task)
			}
			if got := claim.Task.Agent.InheritRuntimeSkills; got == nil || *got != enabled {
				t.Fatalf("claim inheritance = %v, want present and %t", got, enabled)
			}
		})
	}
}

func TestSetAgentRuntimeSkillsInheritedPreservesChoicesAndBroadcasts(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "omp"} {
		t.Run(provider, func(t *testing.T) {
			runtimeID := dbfx.Runtime(t, t.Name(), testutil.Cols{
				"runtime_mode": "local",
				"provider":     provider,
			})
			agentID := dbfx.Agent(t, t.Name(), runtimeID, testutil.Cols{"runtime_mode": "local"})
			disabled := []DisabledRuntimeSkill{{
				RuntimeID: runtimeID, Provider: provider, Root: "provider", Key: "review", Name: "Review",
			}}
			dbfx.Exec(t, `UPDATE agent SET disabled_runtime_skills = $2 WHERE id = $1`, agentID, disabled)
			skillID := insertHandlerTestSkill(t, "inheritance-assigned", "# Assigned skill")
			dbfx.Exec(t, `INSERT INTO agent_skill (agent_id, skill_id) VALUES ($1, $2)`, agentID, skillID)
			t.Cleanup(func() {
				dbfx.Exec(t, `DELETE FROM agent_skill WHERE agent_id = $1`, agentID)
			})

			h := *testHandler
			h.Bus = events.New()
			var broadcasts []AgentResponse
			h.Bus.Subscribe(protocol.EventAgentStatus, func(event events.Event) {
				payload, ok := event.Payload.(map[string]any)
				if !ok {
					t.Fatalf("agent status payload has type %T", event.Payload)
				}
				agent, ok := payload["agent"].(AgentResponse)
				if !ok {
					t.Fatalf("broadcast agent has type %T", payload["agent"])
				}
				broadcasts = append(broadcasts, agent)
			})

			for i, enabled := range []bool{false, true} {
				req := withURLParam(newRequest("PUT", "/api/agents/"+agentID+"/runtime-skills/inherit",
					map[string]any{"enabled": enabled}), "id", agentID)
				testutil.Call(t, h.SetAgentRuntimeSkillsInherited, req).Want(http.StatusNoContent)
				row, err := h.Queries.GetAgent(context.Background(), parseUUID(agentID))
				if err != nil {
					t.Fatalf("load updated agent: %v", err)
				}
				if row.InheritRuntimeSkills != enabled {
					t.Fatalf("stored inheritance = %t, want %t", row.InheritRuntimeSkills, enabled)
				}
				if got := decodeDisabledRuntimeSkills(row.DisabledRuntimeSkills); !slices.Equal(got, disabled) {
					t.Fatalf("section toggle changed per-skill choices: %+v, want %+v", got, disabled)
				}
				if len(broadcasts) != i+1 {
					t.Fatalf("got %d status broadcasts, want %d", len(broadcasts), i+1)
				}
				broadcast := broadcasts[i]
				if broadcast.ID != agentID || broadcast.InheritRuntimeSkills != enabled ||
					!slices.Equal(broadcast.DisabledRuntimeSkills, disabled) {
					t.Fatalf("broadcast does not reflect saved inheritance and choices: %+v", broadcast)
				}
				if len(broadcast.Skills) != 1 || broadcast.Skills[0].ID != skillID {
					t.Fatalf("broadcast lost assigned skills: %+v", broadcast.Skills)
				}
			}
		})
	}
}

func TestSetAgentRuntimeSkillsInheritedRejectsInvalidChanges(t *testing.T) {
	memberID := createRuntimeLocalSkillTestMember(t, "member")
	for _, tc := range []struct {
		name           string
		body           any
		runtimeMode    string
		provider       string
		noRuntime      bool
		foreignRuntime bool
		member         bool
		wantStatus     int
	}{
		{name: "missing enabled", body: map[string]any{}, wantStatus: http.StatusBadRequest},
		{name: "null enabled", body: map[string]any{"enabled": nil}, wantStatus: http.StatusBadRequest},
		{name: "non boolean enabled", body: map[string]any{"enabled": "false"}, wantStatus: http.StatusBadRequest},
		{name: "malformed JSON", body: `{"enabled":`, wantStatus: http.StatusBadRequest},
		{name: "no runtime", noRuntime: true, wantStatus: http.StatusBadRequest},
		{name: "foreign runtime", foreignRuntime: true, wantStatus: http.StatusNotFound},
		{name: "non local runtime", runtimeMode: "cloud", wantStatus: http.StatusBadRequest},
		{name: "unsupported provider", provider: "pi", wantStatus: http.StatusBadRequest},
		{name: "member cannot manage", member: true, wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, provider := tc.runtimeMode, tc.provider
			if mode == "" {
				mode = "local"
			}
			if provider == "" {
				provider = "omp"
			}
			runtimeID := ""
			if !tc.noRuntime {
				cols := testutil.Cols{"runtime_mode": mode, "provider": provider}
				if tc.foreignRuntime {
					cols["workspace_id"] = dbfx.Workspace(t, t.Name(), "foreign-runtime-inheritance")
				}
				runtimeID = dbfx.Runtime(t, t.Name(), cols)
			}
			agentID := dbfx.Agent(t, t.Name(), runtimeID, testutil.Cols{
				"runtime_mode": mode, "visibility": "workspace", "permission_mode": "public_to",
			})
			if tc.member {
				dbfx.Insert(t, "agent_invocation_target", testutil.Cols{
					"agent_id": agentID, "target_type": "member", "target_id": memberID, "created_by": testUserID,
				})
			}
			body := tc.body
			if body == nil {
				body = map[string]any{"enabled": false}
			}
			userID := testUserID
			if tc.member {
				userID = memberID
			}
			req := testutil.WithHeaders(testutil.JSONRequest("PUT", "/api/agents/"+agentID+"/runtime-skills/inherit", body),
				"X-User-ID", userID, "X-Workspace-ID", testWorkspaceID)
			req = withURLParam(req, "id", agentID)
			h := *testHandler
			h.Bus = events.New()
			broadcasts := 0
			h.Bus.Subscribe(protocol.EventAgentStatus, func(events.Event) { broadcasts++ })
			testutil.Call(t, h.SetAgentRuntimeSkillsInherited, req).Want(tc.wantStatus)
			row, err := h.Queries.GetAgent(context.Background(), parseUUID(agentID))
			if err != nil {
				t.Fatalf("load rejected agent: %v", err)
			}
			if !row.InheritRuntimeSkills || len(decodeDisabledRuntimeSkills(row.DisabledRuntimeSkills)) != 0 {
				t.Fatalf("rejected change modified runtime skill settings: %+v", row)
			}
			if broadcasts != 0 {
				t.Fatalf("rejected change broadcast %d status events", broadcasts)
			}
		})
	}
}
