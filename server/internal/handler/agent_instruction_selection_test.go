package handler

import (
	"net/http"
	"slices"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestDecodeInstructionSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		want []string
	}{
		{name: "empty"},
		{name: "invalid JSON", raw: []byte(`[`)},
		{name: "wrong shape", raw: []byte(`{"path":"/notes/a.md"}`)},
		{name: "wrong element type", raw: []byte(`["/notes/a.md",42]`)},
		{name: "null", raw: []byte(`null`)},
		{name: "empty array", raw: []byte(`[]`)},
		{name: "ordered duplicates", raw: []byte(`["/notes/b.md","/skills/persona","/notes/b.md"]`), want: []string{"/notes/b.md", "/skills/persona", "/notes/b.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeInstructionSelection(tc.raw)
			if got == nil || !slices.Equal(got, tc.want) {
				t.Fatalf("decodeInstructionSelection() = %#v, want non-nil %#v", got, tc.want)
			}
		})
	}
}

func TestNormaliseInstructionSelection(t *testing.T) {
	for _, tc := range []struct {
		name    string
		paths   []string
		want    []string
		wantErr bool
	}{
		{name: "nil"},
		{name: "empty", paths: []string{}},
		{name: "trim in order with duplicates", paths: []string{" \t/notes/b.md\n", "/skills/persona", "/notes/b.md"}, want: []string{"/notes/b.md", "/skills/persona", "/notes/b.md"}},
		{name: "root", paths: []string{"/"}, want: []string{"/"}},
		{name: "unicode and spaces", paths: []string{"/notes/你好 world.md"}, want: []string{"/notes/你好 world.md"}},
		{name: "empty path", paths: []string{""}, wantErr: true},
		{name: "blank path", paths: []string{" \n\t"}, wantErr: true},
		{name: "relative path", paths: []string{"/notes/a.md", "notes/b.md"}, wantErr: true},
		{name: "home shortcut", paths: []string{"~/notes/a.md"}, wantErr: true},
		{name: "windows path", paths: []string{`C:\notes\a.md`}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := slices.Clone(tc.paths)
			got, err := normaliseInstructionSelection(tc.paths)
			if tc.wantErr {
				if err == nil || err.Error() != "instruction_selection paths must be absolute" {
					t.Fatalf("normaliseInstructionSelection() error = %v, want absolute-path error", err)
				}
			} else if err != nil || got == nil || !slices.Equal(got, tc.want) {
				t.Fatalf("normaliseInstructionSelection() = %#v, %v, want non-nil %#v", got, err, tc.want)
			}
			if !slices.Equal(tc.paths, original) {
				t.Fatalf("input changed: got %#v, want %#v", tc.paths, original)
			}
		})
	}
}

func TestAgentInstructionSelectionDefaultsToEmptyArray(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	runtimeID := dbfx.Runtime(t, t.Name())
	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", map[string]any{
		"name":       t.Name(),
		"runtime_id": runtimeID,
	})).Want(http.StatusCreated).JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM agent WHERE id = $1`, created.ID)
	if created.InstructionSelection == nil || len(created.InstructionSelection) != 0 {
		t.Fatalf("created instruction_selection = %#v, want []", created.InstructionSelection)
	}

	var fetched AgentResponse
	testutil.Call(t, testHandler.GetAgent, withURLParam(
		newRequest(http.MethodGet, "/api/agents/"+created.ID, nil), "id", created.ID,
	)).Want(http.StatusOK).JSON(&fetched)
	if fetched.InstructionSelection == nil || len(fetched.InstructionSelection) != 0 {
		t.Fatalf("fetched instruction_selection = %#v, want []", fetched.InstructionSelection)
	}
}

func TestAgentInstructionSelectionRoundTrip(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	runtimeID := dbfx.Runtime(t, t.Name())
	want := []string{"/notes/b.md", "/skills/persona"}
	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", map[string]any{
		"name":                  t.Name(),
		"runtime_id":            runtimeID,
		"instruction_selection": []string{" /notes/b.md ", "/skills/persona"},
	})).Want(http.StatusCreated).JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM agent WHERE id = $1`, created.ID)
	if !slices.Equal(created.InstructionSelection, want) {
		t.Fatalf("created instruction_selection = %#v, want %#v", created.InstructionSelection, want)
	}

	var fetched AgentResponse
	testutil.Call(t, testHandler.GetAgent, withURLParam(
		newRequest(http.MethodGet, "/api/agents/"+created.ID, nil), "id", created.ID,
	)).Want(http.StatusOK).JSON(&fetched)
	if !slices.Equal(fetched.InstructionSelection, want) {
		t.Fatalf("fetched instruction_selection = %#v, want %#v", fetched.InstructionSelection, want)
	}

	var preserved AgentResponse
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(
		newRequest(http.MethodPut, "/api/agents/"+created.ID, map[string]any{
			"name": t.Name() + "-renamed",
		}), "id", created.ID,
	)).Want(http.StatusOK).JSON(&preserved)
	if !slices.Equal(preserved.InstructionSelection, want) {
		t.Fatalf("omitted instruction_selection = %#v, want %#v", preserved.InstructionSelection, want)
	}

	for _, paths := range [][]string{{"notes/b.md"}, {""}, {"~/notes/b.md"}} {
		testutil.Call(t, testHandler.UpdateAgent, withURLParam(
			newRequest(http.MethodPut, "/api/agents/"+created.ID, map[string]any{
				"instruction_selection": paths,
			}), "id", created.ID,
		)).Want(http.StatusBadRequest)
		var stored []byte
		dbfx.QueryRow(t, `SELECT instruction_selection FROM agent WHERE id = $1`, created.ID).Scan(&stored)
		if got := decodeInstructionSelection(stored); !slices.Equal(got, want) {
			t.Fatalf("selection after rejected update = %#v, want %#v", got, want)
		}
	}

	want = []string{"/skills/persona", "/notes/b.md", "/skills/persona"}
	var updated AgentResponse
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(
		newRequest(http.MethodPut, "/api/agents/"+created.ID, map[string]any{
			"instruction_selection": []string{" /skills/persona ", "/notes/b.md", "/skills/persona"},
		}), "id", created.ID,
	)).Want(http.StatusOK).JSON(&updated)
	if !slices.Equal(updated.InstructionSelection, want) {
		t.Fatalf("updated instruction_selection = %#v, want %#v", updated.InstructionSelection, want)
	}

	for _, empty := range []any{[]string{}, nil} {
		var cleared AgentResponse
		testutil.Call(t, testHandler.UpdateAgent, withURLParam(
			newRequest(http.MethodPut, "/api/agents/"+created.ID, map[string]any{
				"instruction_selection": empty,
			}), "id", created.ID,
		)).Want(http.StatusOK).JSON(&cleared)
		if cleared.InstructionSelection == nil || len(cleared.InstructionSelection) != 0 {
			t.Fatalf("cleared instruction_selection = %#v, want []", cleared.InstructionSelection)
		}
	}
}

func TestCreateAgentInstructionSelectionRejectsRelativePath(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	runtimeID := dbfx.Runtime(t, t.Name())
	testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", map[string]any{
		"name":                  t.Name(),
		"runtime_id":            runtimeID,
		"instruction_selection": []string{"notes/b.md"},
	})).Want(http.StatusBadRequest)
	if count := dbfx.Count(t, `SELECT count(*) FROM agent WHERE runtime_id = $1`, runtimeID); count != 0 {
		t.Fatalf("agents after rejected create = %d, want 0", count)
	}
}

func TestAgentInstructionSelectionRuntimeOwnerGate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	for _, role := range []string{"member", "admin"} {
		t.Run(role, func(t *testing.T) {
			memberID := dbfx.User(t, "Selection editor", t.Name()+"@multica.test")
			dbfx.Member(t, testWorkspaceID, memberID, role)
			runtimeID := dbfx.Runtime(t, t.Name(), testutil.Cols{"visibility": "public"})
			agentID := dbfx.Agent(t, t.Name(), runtimeID, testutil.Cols{
				"owner_id":        memberID,
				"visibility":      "workspace",
				"permission_mode": "public_to",
			})
			dbfx.InsertNoID(t, "agent_invocation_target", testutil.Cols{
				"agent_id": agentID, "target_type": "workspace", "target_id": testWorkspaceID,
			}, "agent_id = $1", agentID)

			want := []string{"/notes/a.md", "/skills/persona"}
			var ownedUpdate AgentResponse
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(
				newRequest(http.MethodPut, "/api/agents/"+agentID, map[string]any{
					"instruction_selection": want,
				}), "id", agentID,
			)).Want(http.StatusOK).JSON(&ownedUpdate)
			if !slices.Equal(ownedUpdate.InstructionSelection, want) {
				t.Fatalf("runtime owner selection = %#v, want %#v", ownedUpdate.InstructionSelection, want)
			}

			for _, changed := range []any{
				[]string{"/notes/b.md"},
				[]string{"/skills/persona", "/notes/a.md"},
				[]string{"/notes/a.md", "/skills/persona", "/notes/a.md"},
				[]string{},
				nil,
			} {
				var denied map[string]string
				testutil.Call(t, testHandler.UpdateAgent, withURLParam(
					newRequestAs(memberID, http.MethodPut, "/api/agents/"+agentID, map[string]any{
						"instructions":          "must not be saved",
						"instruction_selection": changed,
					}), "id", agentID,
				)).Want(http.StatusForbidden).JSON(&denied)
				if denied["error"] != "only the runtime owner can change instruction_selection" {
					t.Fatalf("selection owner rejection = %#v", denied)
				}
				var stored []byte
				var instructions string
				dbfx.QueryRow(t, `SELECT instruction_selection, instructions FROM agent WHERE id = $1`, agentID).Scan(&stored, &instructions)
				if got := decodeInstructionSelection(stored); !slices.Equal(got, want) || instructions != "" {
					t.Fatalf("rejected update stored selection = %#v, instructions = %q", got, instructions)
				}
			}

			var unchanged AgentResponse
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(
				newRequestAs(memberID, http.MethodPut, "/api/agents/"+agentID, map[string]any{
					"instructions":          "Read first.\n{selection}",
					"instruction_selection": []string{" /notes/a.md ", "/skills/persona"},
				}), "id", agentID,
			)).Want(http.StatusOK).JSON(&unchanged)
			if !slices.Equal(unchanged.InstructionSelection, want) || unchanged.Instructions != "Read first.\n{selection}" {
				t.Fatalf("unchanged-selection update = %#v, instructions = %q", unchanged.InstructionSelection, unchanged.Instructions)
			}

			var omitted AgentResponse
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(
				newRequestAs(memberID, http.MethodPut, "/api/agents/"+agentID, map[string]any{
					"instructions": "Updated text.\n{selection}",
				}), "id", agentID,
			)).Want(http.StatusOK).JSON(&omitted)
			if !slices.Equal(omitted.InstructionSelection, want) || omitted.Instructions != "Updated text.\n{selection}" {
				t.Fatalf("omitted-selection update = %#v, instructions = %q", omitted.InstructionSelection, omitted.Instructions)
			}

			viewerID := dbfx.User(t, "Selection viewer", t.Name()+"-viewer@multica.test")
			dbfx.Member(t, testWorkspaceID, viewerID, "member")
			var fetched AgentResponse
			testutil.Call(t, testHandler.GetAgent, withURLParam(
				newRequestAs(viewerID, http.MethodGet, "/api/agents/"+agentID, nil), "id", agentID,
			)).Want(http.StatusOK).JSON(&fetched)
			if !slices.Equal(fetched.InstructionSelection, want) {
				t.Fatalf("another member read selection = %#v, want %#v", fetched.InstructionSelection, want)
			}
		})
	}
}

func TestAgentInstructionSelectionRuntimeMoveGate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	memberID := dbfx.User(t, "Selection mover", t.Name()+"@multica.test")
	dbfx.Member(t, testWorkspaceID, memberID, "member")
	memberRuntimeID := dbfx.Runtime(t, t.Name()+" member", testutil.Cols{"owner_id": memberID})
	brianRuntimeID := dbfx.Runtime(t, t.Name()+" brian", testutil.Cols{"visibility": "public"})
	agentID := dbfx.Agent(t, t.Name(), memberRuntimeID, testutil.Cols{"owner_id": memberID})
	move := func(userID, runtimeID string, status int) {
		testutil.Call(t, testHandler.UpdateAgent, withURLParam(
			newRequestAs(userID, http.MethodPut, "/api/agents/"+agentID, map[string]any{"runtime_id": runtimeID}), "id", agentID,
		)).Want(status)
	}
	storedRuntime := func() string {
		var runtimeID string
		dbfx.QueryRow(t, `SELECT runtime_id::text FROM agent WHERE id = $1`, agentID).Scan(&runtimeID)
		return runtimeID
	}

	// Without a selection, the move is not gated.
	move(memberID, brianRuntimeID, http.StatusOK)
	move(memberID, memberRuntimeID, http.StatusOK)

	testutil.Call(t, testHandler.UpdateAgent, withURLParam(
		newRequestAs(memberID, http.MethodPut, "/api/agents/"+agentID, map[string]any{"instruction_selection": []string{"/Users/member/.ssh/id_ed25519"}}), "id", agentID,
	)).Want(http.StatusOK)

	var denied map[string]string
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(
		newRequestAs(memberID, http.MethodPut, "/api/agents/"+agentID, map[string]any{"runtime_id": brianRuntimeID}), "id", agentID,
	)).Want(http.StatusForbidden).JSON(&denied)
	if denied["error"] != "only the runtime owner can change instruction_selection" || storedRuntime() != memberRuntimeID {
		t.Fatalf("move with selection = %#v, runtime = %s", denied, storedRuntime())
	}

	// Clearing the selection in the same request is a change on the target runtime too.
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(
		newRequestAs(memberID, http.MethodPut, "/api/agents/"+agentID, map[string]any{"runtime_id": brianRuntimeID, "instruction_selection": []string{}}), "id", agentID,
	)).Want(http.StatusForbidden)

	move(testUserID, brianRuntimeID, http.StatusOK)
	if storedRuntime() != brianRuntimeID {
		t.Fatalf("runtime owner move left runtime %s", storedRuntime())
	}
}

func TestCreateAgentInstructionSelectionRuntimeOwnerGate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	memberID := dbfx.User(t, "Agent creator", t.Name()+"@multica.test")
	dbfx.Member(t, testWorkspaceID, memberID, "member")
	runtimeID := dbfx.Runtime(t, t.Name(), testutil.Cols{"visibility": "public"})
	var denied map[string]string
	testutil.Call(t, testHandler.CreateAgent, newRequestAs(memberID, http.MethodPost, "/api/agents", map[string]any{
		"name":                  t.Name(),
		"runtime_id":            runtimeID,
		"instruction_selection": []string{"/notes/a.md"},
	})).Want(http.StatusForbidden).JSON(&denied)
	if denied["error"] != "only the runtime owner can change instruction_selection" {
		t.Fatalf("selection owner rejection = %#v", denied)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent WHERE runtime_id = $1`, runtimeID); count != 0 {
		t.Fatalf("agents after rejected selection create = %d, want 0", count)
	}

	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgent, newRequestAs(memberID, http.MethodPost, "/api/agents", map[string]any{
		"name":                  t.Name(),
		"runtime_id":            runtimeID,
		"instruction_selection": []string{},
	})).Want(http.StatusCreated).JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM agent WHERE id = $1`, created.ID)
	if created.InstructionSelection == nil || len(created.InstructionSelection) != 0 {
		t.Fatalf("non-owner empty selection create = %#v, want []", created.InstructionSelection)
	}
}
