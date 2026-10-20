package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

type squadSkillsCall struct {
	method string
	path   string
	body   map[string]any
}

// serveSquadSkills records every request and answers with an empty skill list
// or 204 for the runtime skill routes.
func serveSquadSkills(t *testing.T) *[]squadSkillsCall {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Setenv("MULTICA_WORKSPACE_ID", "workspace-123")
	var got []squadSkillsCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := squadSkillsCall{method: r.Method, path: r.URL.Path}
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req.body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
		}
		got = append(got, req)
		switch r.URL.Path {
		case "/api/squads/squad-123/runtime-skills/enabled", "/api/squads/squad-123/runtime-skills/inherit":
			w.WriteHeader(http.StatusNoContent)
		case "/api/squads/squad-123":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                     "squad-123",
				"inherit_runtime_skills": false,
				"runtime_skill_exceptions": []map[string]any{{
					"runtime_id": "runtime-1", "provider": "omp", "root": "provider", "key": "review",
				}},
			})
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	return &got
}

func newSquadSkillsTestCmd(flags func(*cobra.Command)) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("server-url", "", "")
	cmd.Flags().String("workspace-id", "", "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().String("output", "json", "")
	if flags != nil {
		flags(cmd)
	}
	return cmd
}

func wantSquadSkillsRequest(t *testing.T, got []squadSkillsCall, want squadSkillsCall) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("requests = %+v, want one", got)
	}
	if got[0].method != want.method || got[0].path != want.path {
		t.Fatalf("request = %s %s, want %s %s", got[0].method, got[0].path, want.method, want.path)
	}
	if want.body != nil && !reflect.DeepEqual(got[0].body, want.body) {
		t.Fatalf("body = %#v, want %#v", got[0].body, want.body)
	}
}

func TestSquadSkillsAssignedCommands(t *testing.T) {
	withSkillIDs := func(ids string) func(*cobra.Command) {
		return func(cmd *cobra.Command) {
			cmd.Flags().StringSlice("skill-ids", nil, "")
			_ = cmd.Flags().Set("skill-ids", ids)
		}
	}
	for _, tc := range []struct {
		name  string
		run   func(*cobra.Command, []string) error
		flags func(*cobra.Command)
		args  []string
		want  squadSkillsCall
	}{
		{name: "list", run: runSquadSkillsList, args: []string{"squad-123"},
			want: squadSkillsCall{method: http.MethodGet, path: "/api/squads/squad-123/skills"}},
		{name: "set", run: runSquadSkillsSet, flags: withSkillIDs("skill-1, skill-2"), args: []string{"squad-123"},
			want: squadSkillsCall{method: http.MethodPut, path: "/api/squads/squad-123/skills",
				body: map[string]any{"skill_ids": []any{"skill-1", "skill-2"}}}},
		{name: "add", run: runSquadSkillsAdd, flags: withSkillIDs("skill-3"), args: []string{"squad-123"},
			want: squadSkillsCall{method: http.MethodPost, path: "/api/squads/squad-123/skills/add",
				body: map[string]any{"skill_ids": []any{"skill-3"}}}},
		{name: "remove", run: runSquadSkillsRemove, args: []string{"squad-123", "skill-1"},
			want: squadSkillsCall{method: http.MethodDelete, path: "/api/squads/squad-123/skills/skill-1"}},
		{name: "enable", run: runSquadSkillsEnable, args: []string{"squad-123", "skill-1"},
			want: squadSkillsCall{method: http.MethodPut, path: "/api/squads/squad-123/skills/skill-1/enabled",
				body: map[string]any{"enabled": true}}},
		{name: "disable", run: runSquadSkillsDisable, args: []string{"squad-123", "skill-1"},
			want: squadSkillsCall{method: http.MethodPut, path: "/api/squads/squad-123/skills/skill-1/enabled",
				body: map[string]any{"enabled": false}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := serveSquadSkills(t)
			if err := tc.run(newSquadSkillsTestCmd(tc.flags), tc.args); err != nil {
				t.Fatalf("run: %v", err)
			}
			wantSquadSkillsRequest(t, *got, tc.want)
		})
	}
}

func TestSquadSkillsSetRequiresSkillIDs(t *testing.T) {
	got := serveSquadSkills(t)
	cmd := newSquadSkillsTestCmd(func(cmd *cobra.Command) { cmd.Flags().StringSlice("skill-ids", nil, "") })
	if err := runSquadSkillsSet(cmd, []string{"squad-123"}); err == nil {
		t.Fatal("expected missing --skill-ids error")
	}
	if len(*got) != 0 {
		t.Fatalf("refused command sent %d requests", len(*got))
	}
}

func TestSquadSkillsRuntimeCommands(t *testing.T) {
	withIdentity := func(cmd *cobra.Command) {
		addSquadRuntimeSkillFlags(cmd)
		_ = cmd.Flags().Set("runtime-id", "runtime-1")
		_ = cmd.Flags().Set("key", "review")
		_ = cmd.Flags().Set("name", "Review")
	}
	identity := func(enabled bool) map[string]any {
		return map[string]any{
			"runtime_id": "runtime-1", "root": "provider", "key": "review", "name": "Review", "plugin": "", "enabled": enabled,
		}
	}
	for _, tc := range []struct {
		name  string
		run   func(*cobra.Command, []string) error
		flags func(*cobra.Command)
		args  []string
		want  squadSkillsCall
	}{
		{name: "list", run: runSquadRuntimeSkillsList, args: []string{"squad-123"},
			want: squadSkillsCall{method: http.MethodGet, path: "/api/squads/squad-123"}},
		{name: "enable", run: runSquadRuntimeSkillEnable, flags: withIdentity, args: []string{"squad-123"},
			want: squadSkillsCall{method: http.MethodPut, path: "/api/squads/squad-123/runtime-skills/enabled", body: identity(true)}},
		{name: "disable", run: runSquadRuntimeSkillDisable, flags: withIdentity, args: []string{"squad-123"},
			want: squadSkillsCall{method: http.MethodPut, path: "/api/squads/squad-123/runtime-skills/enabled", body: identity(false)}},
		{name: "section on", run: runSquadRuntimeSkillsSection, args: []string{"squad-123", "on"},
			want: squadSkillsCall{method: http.MethodPut, path: "/api/squads/squad-123/runtime-skills/inherit",
				body: map[string]any{"enabled": true}}},
		{name: "section off", run: runSquadRuntimeSkillsSection, args: []string{"squad-123", "off"},
			want: squadSkillsCall{method: http.MethodPut, path: "/api/squads/squad-123/runtime-skills/inherit",
				body: map[string]any{"enabled": false}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := serveSquadSkills(t)
			if err := tc.run(newSquadSkillsTestCmd(tc.flags), tc.args); err != nil {
				t.Fatalf("run: %v", err)
			}
			wantSquadSkillsRequest(t, *got, tc.want)
		})
	}
}

func TestSquadSkillsRuntimeCommandsValidateInput(t *testing.T) {
	got := serveSquadSkills(t)
	if err := runSquadRuntimeSkillsSection(newSquadSkillsTestCmd(nil), []string{"squad-123", "maybe"}); err == nil {
		t.Fatal("expected on/off error")
	}
	cmd := newSquadSkillsTestCmd(func(cmd *cobra.Command) {
		addSquadRuntimeSkillFlags(cmd)
		_ = cmd.Flags().Set("key", "review")
	})
	if err := runSquadRuntimeSkillEnable(cmd, []string{"squad-123"}); err == nil {
		t.Fatal("expected missing --runtime-id error")
	}
	cmd = newSquadSkillsTestCmd(func(cmd *cobra.Command) {
		addSquadRuntimeSkillFlags(cmd)
		_ = cmd.Flags().Set("runtime-id", "runtime-1")
	})
	if err := runSquadRuntimeSkillDisable(cmd, []string{"squad-123"}); err == nil {
		t.Fatal("expected missing --key error")
	}
	if len(*got) != 0 {
		t.Fatalf("refused commands sent %d requests", len(*got))
	}
}
