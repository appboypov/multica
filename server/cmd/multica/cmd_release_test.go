package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func executeReleaseTest(t *testing.T, server string, args ...string) (string, error) {
	t.Helper()
	cmd := newReleaseCommand()
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.PersistentFlags().String("server-url", server, "")
	cmd.PersistentFlags().String("workspace-id", "ws-1", "")
	cmd.PersistentFlags().String("profile", "", "")
	if err := cmd.PersistentFlags().Set("server-url", server); err != nil {
		t.Fatal(err)
	}
	if err := cmd.PersistentFlags().Set("workspace-id", "ws-1"); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs(args)
	return captureStdout(t, func() error { return cmd.Execute() })
}

func TestReleaseMilestoneCLIAddGetEditRemove(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "test-token")
	const releaseID = "11111111-1111-4111-8111-111111111111"
	const milestoneID = "22222222-2222-4222-8222-222222222222"
	release := releaseDTO{ID: releaseID, RepositoryID: "repo-1", Version: "1.0.0", Title: "First", Milestones: []releaseMilestoneDTO{}}
	var writes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Errorf("workspace header = %q", r.Header.Get("X-Workspace-ID"))
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/releases":
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{release}})
		case "GET /api/releases/" + releaseID:
			json.NewEncoder(w).Encode(release)
		case "POST /api/releases/" + releaseID + "/milestones":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if !reflect.DeepEqual(body, map[string]string{"name": "UI prototype", "date": "2026-11-02"}) {
				t.Errorf("add body = %#v", body)
			}
			release.Milestones = []releaseMilestoneDTO{{ID: milestoneID, ReleaseID: releaseID, Name: body["name"], Date: body["date"]}}
			writes = append(writes, "add")
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{release}})
		case "PATCH /api/release-milestones/" + milestoneID:
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if !reflect.DeepEqual(body, map[string]string{"date": "2026-11-03"}) {
				t.Errorf("edit body = %#v", body)
			}
			release.Milestones[0].Date = body["date"]
			writes = append(writes, "edit")
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{release}})
		case "DELETE /api/release-milestones/" + milestoneID:
			release.Milestones = []releaseMilestoneDTO{}
			writes = append(writes, "remove")
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{release}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	if _, err := executeReleaseTest(t, srv.URL, "milestone", "add", releaseID, "--name", "UI prototype", "--date", "2026-11-02"); err != nil {
		t.Fatal(err)
	}
	out, err := executeReleaseTest(t, srv.URL, "get", "1111", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got releaseDTO
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Milestones) != 1 || got.Milestones[0].Name != "UI prototype" || got.Milestones[0].Date != "2026-11-02" {
		t.Fatalf("get output = %s", out)
	}
	if _, err := executeReleaseTest(t, srv.URL, "milestone", "edit", "2222", "--date", "2026-11-03"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeReleaseTest(t, srv.URL, "milestone", "remove", milestoneID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writes, []string{"add", "edit", "remove"}) {
		t.Fatalf("writes = %v", writes)
	}
}

func TestReleaseCLIExistingOperations(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "test-token")
	const releaseID = "11111111-1111-4111-8111-111111111111"
	const repoID = "33333333-3333-4333-8333-333333333333"
	const projectID = "44444444-4444-4444-8444-444444444444"
	const issueID = "55555555-5555-4555-8555-555555555555"
	rel := releaseDTO{ID: releaseID, RepositoryID: repoID, Version: "1.0.0", Title: "Before", Description: "Keep", Milestones: []releaseMilestoneDTO{}}
	var writes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Method != "GET" {
			json.NewDecoder(r.Body).Decode(&body)
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/repositories":
			json.NewEncoder(w).Encode(map[string]any{"repositories": []issuePlacementRepository{{ID: repoID, FullName: "owner/repo", ProjectID: projectID}}})
		case "GET /api/releases":
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{rel, {ID: "other", RepositoryID: "other", Version: "2.0.0", Title: "Other"}}})
		case "GET /api/releases/" + releaseID:
			json.NewEncoder(w).Encode(rel)
		case "POST /api/repositories/" + repoID + "/releases":
			if body["bump"] != "minor" || body["title"] != "Next" {
				t.Errorf("plan body = %#v", body)
			}
			writes = append(writes, "plan")
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{rel}})
		case "PATCH /api/releases/" + releaseID:
			if body["version"] != "1.0.0" || body["description"] != "Keep" || body["title"] != "After" {
				t.Errorf("edit body = %#v", body)
			}
			writes = append(writes, "edit")
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{rel}})
		case "POST /api/releases/" + releaseID + "/move":
			if body["direction"] != "later" {
				t.Errorf("move body = %#v", body)
			}
			writes = append(writes, "move")
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{rel}})
		case "DELETE /api/releases/" + releaseID:
			writes = append(writes, "remove")
			json.NewEncoder(w).Encode(releaseListDTO{[]releaseDTO{}})
		case "POST /api/releases/" + releaseID + "/issues":
			if !reflect.DeepEqual(body["issue_ids"], []any{issueID}) {
				t.Errorf("issue body = %#v", body)
			}
			writes = append(writes, "add-issues")
			json.NewEncoder(w).Encode(map[string]any{"issues": []any{}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	for _, flags := range [][]string{{"--repository", "owner/repo"}, {"--project", projectID}, {"--repository", "3333", "--project", projectID}} {
		args := append([]string{"list", "--output", "json"}, flags...)
		out, err := executeReleaseTest(t, srv.URL, args...)
		if err != nil {
			t.Fatal(err)
		}
		var rows []releaseDTO
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].ID != releaseID {
			t.Fatalf("filtered list = %s", out)
		}
	}
	for _, args := range [][]string{
		{"plan", "owner/repo", "--title", "Next", "--bump", "minor"},
		{"edit", releaseID, "--title", "After"}, {"move", releaseID, "--direction", "later"},
		{"add-issues", releaseID, issueID}, {"remove", releaseID},
	} {
		if _, err := executeReleaseTest(t, srv.URL, args...); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(writes, []string{"plan", "edit", "move", "add-issues", "remove"}) {
		t.Fatalf("writes = %v", writes)
	}
}

func TestReleaseCLIRejectsInvalidMilestoneFields(t *testing.T) {
	for _, args := range [][]string{
		{"milestone", "add", "release", "--name", "UI"},
		{"milestone", "add", "release", "--name", " ", "--date", "2026-11-02"},
		{"milestone", "add", "release", "--name", "UI", "--date", "2026-02-30"},
		{"milestone", "edit", "milestone"}, {"milestone", "edit", "milestone", "--date", "2026-1-2"},
	} {
		if _, err := executeReleaseTest(t, "http://127.0.0.1:1", args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	root := newReleaseCommand()
	if cmd, _, err := root.Find([]string{"ship"}); err == nil && cmd != root {
		t.Fatal("shipping is not implemented by the API")
	}
}
