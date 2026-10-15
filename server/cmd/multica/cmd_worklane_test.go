package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

func laneTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	for _, name := range []string{"repo", "project", "folder", "daemon-id", "before", "after", "server-url", "workspace-id", "profile"} {
		cmd.Flags().String(name, "", "")
	}
	cmd.Flags().String("output", "json", "")
	return cmd
}

func TestWorklaneMoveUsesIssueResolversAndLaneEndpoint(t *testing.T) {
	t.Chdir(t.TempDir())
	const lane = "11111111-1111-4111-8111-111111111111"
	const moving = "22222222-2222-4222-8222-222222222222"
	const other = "33333333-3333-4333-8333-333333333333"
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/issues/MUL-1", "/api/issues/" + moving:
			json.NewEncoder(w).Encode(map[string]any{"id": moving, "identifier": "MUL-1", "worklane_id": lane})
		case "/api/issues/MUL-2":
			json.NewEncoder(w).Encode(map[string]any{"id": other, "identifier": "MUL-2"})
		case "/api/worklanes/" + lane + "/issues/" + moving + "/move":
			writes++
			if r.Method != "POST" {
				t.Errorf("method = %s", r.Method)
			}
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["before"] != other || body["after"] != "" {
				t.Errorf("body = %v", body)
			}
			json.NewEncoder(w).Encode(map[string]any{"issues": []any{map[string]any{"id": moving, "identifier": "MUL-1"}, map[string]any{"id": other, "identifier": "MUL-2"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws")
	t.Setenv("MULTICA_TOKEN", "test")
	cmd := laneTestCommand()
	cmd.Flags().Set("before", "MUL-2")
	output, err := captureStdout(t, func() error { return runWorklaneMove(cmd, []string{"MUL-1"}) })
	if err != nil || writes != 1 {
		t.Fatalf("move err=%v, writes=%d", err, writes)
	}
	if strings.Index(output, "MUL-1") > strings.Index(output, "MUL-2") {
		t.Fatalf("wrong order: %s", output)
	}
}

func TestWorklaneCLIReadAndMutationCommands(t *testing.T) {
	t.Chdir(t.TempDir())
	lanes := []map[string]any{{"id": "lane-a", "name": "main", "repository_id": "repo-a", "state": "free"}, {"id": "lane-b", "name": "main", "repository_id": "repo-b", "state": "free"}}
	repositories := []map[string]string{{"id": "repo-a", "full_name": "test/a", "project_id": "project-a"}, {"id": "repo-b", "full_name": "test/b", "project_id": "project-b"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/worklanes":
			json.NewEncoder(w).Encode(map[string]any{"worklanes": lanes})
		case "/api/repositories":
			json.NewEncoder(w).Encode(map[string]any{"repositories": repositories})
		case "/api/worklanes/lane-a/issues":
			json.NewEncoder(w).Encode(map[string]any{"issues": []any{map[string]string{"id": "second", "identifier": "MUL-2"}, map[string]string{"id": "first", "identifier": "MUL-1"}}})
		case "/api/worklanes/lane-a":
			switch r.Method {
			case "DELETE":
				json.NewEncoder(w).Encode(map[string]string{"id": "request", "status": "completed", "worklane_id": "lane-a"})
			case "PATCH":
				var body map[string]string
				json.NewDecoder(r.Body).Decode(&body)
				json.NewEncoder(w).Encode(map[string]string{"id": "lane-a", "name": body["name"]})
			default:
				json.NewEncoder(w).Encode(lanes[0])
			}
		case "/api/worklane-requests":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["repository_id"] != "repo-a" || body["folder"] != "work" || body["daemon_id"] != "machine" {
				t.Errorf("create body = %v", body)
			}
			json.NewEncoder(w).Encode(map[string]string{"id": "request", "status": "completed", "worklane_id": "new-lane"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws")
	t.Setenv("MULTICA_TOKEN", "test")
	client := cli.NewAPIClient(srv.URL, "ws", "test")
	if _, err := resolveCommandWorklane(context.Background(), client, "main", ""); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous name err = %v", err)
	}
	for _, tc := range []struct {
		name     string
		run      func(*cobra.Command, []string) error
		args     []string
		contains string
	}{
		{"list", runWorklaneList, nil, "lane-a"},
		{"get", runWorklaneGet, []string{"main"}, "lane-a"},
		{"issues", runWorklaneIssues, []string{"main"}, "MUL-2"},
		{"rename", runWorklaneRename, []string{"main", "new name"}, "new name"},
		{"remove", runWorklaneRemove, []string{"main"}, "completed"},
		{"create", runWorklaneCreate, []string{"Work"}, "new-lane"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := laneTestCommand()
			cmd.Flags().Set("repo", "test/a")
			cmd.Flags().Set("folder", "work")
			cmd.Flags().Set("daemon-id", "machine")
			output, err := captureStdout(t, func() error { return tc.run(cmd, tc.args) })
			if err != nil || !strings.Contains(output, tc.contains) {
				t.Fatalf("output=%s, err=%v", output, err)
			}
			if tc.name == "list" && strings.Contains(output, "lane-b") {
				t.Fatalf("repository filter ignored: %s", output)
			}
			if tc.name == "issues" && strings.Index(output, "MUL-2") > strings.Index(output, "MUL-1") {
				t.Fatalf("server order lost: %s", output)
			}
		})
	}
}
