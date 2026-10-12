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
