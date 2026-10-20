package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// squadSkillsHandler returns a handler copy with its own event bus and the
// count of squad:updated events it publishes.
func squadSkillsHandler() (*Handler, *int) {
	h := *testHandler
	h.Bus = events.New()
	broadcasts := 0
	h.Bus.Subscribe(protocol.EventSquadUpdated, func(events.Event) { broadcasts++ })
	return &h, &broadcasts
}

func callSquadSkills(t *testing.T, handle http.HandlerFunc, userID, method, path string, body any, params map[string]string, want int) []SkillSummaryResponse {
	t.Helper()
	w := httptest.NewRecorder()
	handle(w, squadScopeReq(userID, method, path, body, params))
	if w.Code != want {
		t.Fatalf("%s %s: status = %d, want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	if want != http.StatusOK {
		return nil
	}
	var skills []SkillSummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&skills); err != nil {
		t.Fatalf("decode squad skills: %v", err)
	}
	return skills
}

func squadSkillStates(skills []SkillSummaryResponse) map[string]bool {
	states := make(map[string]bool, len(skills))
	for _, skill := range skills {
		states[skill.ID] = skill.Enabled != nil && *skill.Enabled
	}
	return states
}

func wantSquadSkillStates(t *testing.T, skills []SkillSummaryResponse, want map[string]bool) {
	t.Helper()
	got := squadSkillStates(skills)
	if len(got) != len(want) {
		t.Fatalf("squad skills = %v, want %v", got, want)
	}
	for id, enabled := range want {
		if state, ok := got[id]; !ok || state != enabled {
			t.Fatalf("squad skills = %v, want %v", got, want)
		}
	}
}

func TestSquadAssignedSkills(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	squad := createSquadAs(t, "", "Squad Skills", createHandlerTestAgent(t, "squad-skills-leader", nil))
	dbfx.Cleanup(t, `DELETE FROM squad_skill WHERE squad_id = $1`, squad.ID)
	review := insertHandlerTestSkill(t, "squad-review", "# Review")
	deploy := insertHandlerTestSkill(t, "squad-deploy", "# Deploy")
	h, broadcasts := squadSkillsHandler()
	params := map[string]string{"id": squad.ID}

	skills := callSquadSkills(t, h.SetSquadSkills, "", "PUT", "/api/squads/"+squad.ID+"/skills",
		map[string]any{"skill_ids": []string{review}}, params, http.StatusOK)
	wantSquadSkillStates(t, skills, map[string]bool{review: true})

	skills = callSquadSkills(t, h.AddSquadSkills, "", "POST", "/api/squads/"+squad.ID+"/skills/add",
		map[string]any{"skill_ids": []string{deploy}}, params, http.StatusOK)
	wantSquadSkillStates(t, skills, map[string]bool{review: true, deploy: true})

	skills = callSquadSkills(t, h.SetSquadSkillEnabled, "", "PUT", "/api/squads/"+squad.ID+"/skills/"+deploy+"/enabled",
		map[string]any{"enabled": false}, map[string]string{"id": squad.ID, "skillId": deploy}, http.StatusOK)
	wantSquadSkillStates(t, skills, map[string]bool{review: true, deploy: false})

	skills = callSquadSkills(t, h.RemoveSquadSkill, "", "DELETE", "/api/squads/"+squad.ID+"/skills/"+review,
		nil, map[string]string{"id": squad.ID, "skillId": review}, http.StatusOK)
	wantSquadSkillStates(t, skills, map[string]bool{deploy: false})

	skills = callSquadSkills(t, h.ListSquadSkills, "", "GET", "/api/squads/"+squad.ID+"/skills", nil, params, http.StatusOK)
	wantSquadSkillStates(t, skills, map[string]bool{deploy: false})
	if *broadcasts != 4 {
		t.Fatalf("got %d squad:updated broadcasts, want 4", *broadcasts)
	}

	// A deleted skill leaves the squad with it.
	if err := testHandler.Queries.DeleteSkill(context.Background(), db.DeleteSkillParams{
		ID: parseUUID(deploy), WorkspaceID: parseUUID(testWorkspaceID),
	}); err != nil {
		t.Fatalf("delete skill: %v", err)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM squad_skill WHERE skill_id = $1`, deploy); n != 0 {
		t.Fatalf("deleted skill kept %d squad assignments", n)
	}
}

func TestSquadAssignedSkillsRefusals(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	squad := createSquadAs(t, "", "Squad Skills Refusals", createHandlerTestAgent(t, "squad-skills-refusal-leader", nil))
	dbfx.Cleanup(t, `DELETE FROM squad_skill WHERE squad_id = $1`, squad.ID)
	review := insertHandlerTestSkill(t, "squad-refusal-review", "# Review")
	memberID := createPlainMember(t, "squad-skills-member@multica.test")
	foreignWorkspace := dbfx.Workspace(t, t.Name(), "squad-skills-foreign")
	foreignSkill := dbfx.Insert(t, "skill", testutil.Cols{"workspace_id": foreignWorkspace, "name": "foreign-" + t.Name()})
	h, broadcasts := squadSkillsHandler()
	params := map[string]string{"id": squad.ID}
	path := "/api/squads/" + squad.ID + "/skills"

	callSquadSkills(t, h.SetSquadSkills, memberID, "PUT", path,
		map[string]any{"skill_ids": []string{review}}, params, http.StatusForbidden)
	callSquadSkills(t, h.SetSquadSkills, "", "PUT", path,
		map[string]any{"skill_ids": []string{foreignSkill}}, params, http.StatusNotFound)
	callSquadSkills(t, h.SetSquadSkillEnabled, "", "PUT", path+"/"+review+"/enabled",
		map[string]any{"enabled": false}, map[string]string{"id": squad.ID, "skillId": review}, http.StatusNotFound)
	callSquadSkills(t, h.SetSquadSkillEnabled, "", "PUT", path+"/"+review+"/enabled",
		map[string]any{}, map[string]string{"id": squad.ID, "skillId": review}, http.StatusBadRequest)

	if n := dbfx.Count(t, `SELECT count(*) FROM squad_skill WHERE squad_id = $1`, squad.ID); n != 0 {
		t.Fatalf("refused changes stored %d squad skills", n)
	}
	if *broadcasts != 0 {
		t.Fatalf("refused changes broadcast %d squad:updated events", *broadcasts)
	}
}

func getSquadRow(t *testing.T, squadID string) db.Squad {
	t.Helper()
	row, err := testHandler.Queries.GetSquad(context.Background(), parseUUID(squadID))
	if err != nil {
		t.Fatalf("load squad: %v", err)
	}
	return row
}

func TestSquadRuntimeSkillsStartOff(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	squad := createSquadAs(t, "", "Squad Runtime Default", createHandlerTestAgent(t, "squad-runtime-default-leader", nil))
	if squad.InheritRuntimeSkills || squad.RuntimeSkillExceptions == nil || len(squad.RuntimeSkillExceptions) != 0 {
		t.Fatalf("new squad runtime skills = %t %v, want off with no exceptions",
			squad.InheritRuntimeSkills, squad.RuntimeSkillExceptions)
	}
}

func TestSetSquadRuntimeSkillEnabledStoresExceptionsAgainstTheSwitch(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	for _, provider := range []string{"claude", "codex", "omp"} {
		t.Run(provider, func(t *testing.T) {
			runtimeID := dbfx.Runtime(t, t.Name(), testutil.Cols{"runtime_mode": "local", "provider": provider})
			squad := createSquadAs(t, "", "Squad Runtime "+provider, createHandlerTestAgent(t, "squad-runtime-"+provider, nil))
			h, broadcasts := squadSkillsHandler()
			params := map[string]string{"id": squad.ID}
			set := func(enabled bool) {
				t.Helper()
				callSquadSkills(t, h.SetSquadRuntimeSkillEnabled, "", "PUT", "/api/squads/"+squad.ID+"/runtime-skills/enabled",
					map[string]any{"runtime_id": runtimeID, "root": "provider", "key": "review", "name": "Review", "enabled": enabled},
					params, http.StatusNoContent)
			}
			want := RuntimeSkillException{RuntimeID: runtimeID, Provider: provider, Root: "provider", Key: "review", Name: "Review"}

			// Section off: turning a skill on is the exception.
			set(true)
			got := decodeRuntimeSkillExceptions(getSquadRow(t, squad.ID).RuntimeSkillExceptions)
			if len(got) != 1 || got[0] != want {
				t.Fatalf("exceptions = %+v, want [%+v]", got, want)
			}
			set(false)
			if got := decodeRuntimeSkillExceptions(getSquadRow(t, squad.ID).RuntimeSkillExceptions); len(got) != 0 {
				t.Fatalf("skill back to the section state kept exceptions: %+v", got)
			}

			// Section on: turning a skill off is the exception.
			dbfx.Exec(t, `UPDATE squad SET inherit_runtime_skills = true WHERE id = $1`, squad.ID)
			set(false)
			got = decodeRuntimeSkillExceptions(getSquadRow(t, squad.ID).RuntimeSkillExceptions)
			if len(got) != 1 || got[0] != want {
				t.Fatalf("exceptions = %+v, want [%+v]", got, want)
			}
			if *broadcasts != 3 {
				t.Fatalf("got %d squad:updated broadcasts, want 3", *broadcasts)
			}
		})
	}
}

func TestSetSquadRuntimeSkillsInheritedClearsExceptions(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, t.Name(), testutil.Cols{"runtime_mode": "local", "provider": "omp"})
	squad := createSquadAs(t, "", "Squad Runtime Flip", createHandlerTestAgent(t, "squad-runtime-flip", nil))
	exceptions := []RuntimeSkillException{{RuntimeID: runtimeID, Provider: "omp", Root: "provider", Key: "review"}}
	review := insertHandlerTestSkill(t, "squad-flip-review", "# Review")
	dbfx.Exec(t, `INSERT INTO squad_skill (squad_id, skill_id) VALUES ($1, $2)`, squad.ID, review)
	dbfx.Cleanup(t, `DELETE FROM squad_skill WHERE squad_id = $1`, squad.ID)
	h, broadcasts := squadSkillsHandler()

	for i, enabled := range []bool{true, false} {
		dbfx.Exec(t, `UPDATE squad SET runtime_skill_exceptions = $2 WHERE id = $1`, squad.ID, exceptions)
		callSquadSkills(t, h.SetSquadRuntimeSkillsInherited, "", "PUT", "/api/squads/"+squad.ID+"/runtime-skills/inherit",
			map[string]any{"enabled": enabled}, map[string]string{"id": squad.ID}, http.StatusNoContent)
		row := getSquadRow(t, squad.ID)
		if row.InheritRuntimeSkills != enabled {
			t.Fatalf("stored section switch = %t, want %t", row.InheritRuntimeSkills, enabled)
		}
		if got := decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions); len(got) != 0 {
			t.Fatalf("section flip kept exceptions: %+v", got)
		}
		if *broadcasts != i+1 {
			t.Fatalf("got %d squad:updated broadcasts, want %d", *broadcasts, i+1)
		}
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM squad_skill WHERE squad_id = $1`, squad.ID); n != 1 {
		t.Fatalf("section flip changed assigned skills: %d rows", n)
	}
}

func TestSquadRuntimeSkillRefusals(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	memberID := createPlainMember(t, "squad-runtime-member@multica.test")
	for _, tc := range []struct {
		name           string
		body           map[string]any
		runtimeMode    string
		provider       string
		foreignRuntime bool
		// othersPrivateRuntime puts the runtime on another member's private
		// machine, which the squad manager may neither read nor use.
		othersPrivateRuntime bool
		member               bool
		wantStatus           int
	}{
		{name: "missing enabled", body: map[string]any{"enabled": nil}, wantStatus: http.StatusBadRequest},
		{name: "missing runtime", body: map[string]any{"runtime_id": ""}, wantStatus: http.StatusBadRequest},
		{name: "foreign runtime", foreignRuntime: true, wantStatus: http.StatusNotFound},
		{name: "another member's private runtime", othersPrivateRuntime: true, wantStatus: http.StatusNotFound},
		{name: "non local runtime", runtimeMode: "cloud", wantStatus: http.StatusBadRequest},
		{name: "non enforcing provider", provider: "pi", wantStatus: http.StatusBadRequest},
		{name: "invalid root", body: map[string]any{"root": "home"}, wantStatus: http.StatusBadRequest},
		{name: "plugin root outside claude", body: map[string]any{"root": "plugin", "plugin": "tools"}, wantStatus: http.StatusBadRequest},
		{name: "escaping key", body: map[string]any{"key": "../review"}, wantStatus: http.StatusBadRequest},
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
			cols := testutil.Cols{"runtime_mode": mode, "provider": provider}
			if tc.foreignRuntime {
				cols["workspace_id"] = dbfx.Workspace(t, t.Name(), "squad-runtime-foreign")
			}
			if tc.othersPrivateRuntime {
				cols["owner_id"] = memberID
			}
			runtimeID := dbfx.Runtime(t, t.Name(), cols)
			squad := createSquadAs(t, "", "Squad Runtime Refusal", createHandlerTestAgent(t, "squad-runtime-refusal", nil))
			body := map[string]any{"runtime_id": runtimeID, "root": "provider", "key": "review", "enabled": true}
			for k, v := range tc.body {
				body[k] = v
			}
			userID := ""
			if tc.member {
				userID = memberID
			}
			h, broadcasts := squadSkillsHandler()
			callSquadSkills(t, h.SetSquadRuntimeSkillEnabled, userID, "PUT", "/api/squads/"+squad.ID+"/runtime-skills/enabled",
				body, map[string]string{"id": squad.ID}, tc.wantStatus)
			row := getSquadRow(t, squad.ID)
			if row.InheritRuntimeSkills || len(decodeRuntimeSkillExceptions(row.RuntimeSkillExceptions)) != 0 {
				t.Fatalf("refused change modified runtime skill settings: %+v", row)
			}
			if *broadcasts != 0 {
				t.Fatalf("refused change broadcast %d squad:updated events", *broadcasts)
			}
		})
	}

	t.Run("section switch", func(t *testing.T) {
		squad := createSquadAs(t, "", "Squad Runtime Switch Refusal", createHandlerTestAgent(t, "squad-runtime-switch-refusal", nil))
		h, broadcasts := squadSkillsHandler()
		params := map[string]string{"id": squad.ID}
		path := "/api/squads/" + squad.ID + "/runtime-skills/inherit"
		callSquadSkills(t, h.SetSquadRuntimeSkillsInherited, "", "PUT", path, map[string]any{}, params, http.StatusBadRequest)
		callSquadSkills(t, h.SetSquadRuntimeSkillsInherited, memberID, "PUT", path, map[string]any{"enabled": true}, params, http.StatusForbidden)
		if getSquadRow(t, squad.ID).InheritRuntimeSkills {
			t.Fatal("refused section switch was stored")
		}
		if *broadcasts != 0 {
			t.Fatalf("refused section switch broadcast %d squad:updated events", *broadcasts)
		}
	})
}
