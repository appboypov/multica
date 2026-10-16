package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseAssignUsesWorkspaceAgentSquadResolver(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Chdir(t.TempDir())
	const releaseID = "11111111-1111-4111-8111-111111111111"
	const agentID = "22222222-2222-4222-8222-222222222222"
	const squadID = "33333333-3333-4333-8333-333333333333"
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/agents":
			json.NewEncoder(w).Encode([]map[string]any{{"id": agentID, "name": "Pieter"}, {"id": "44444444-4444-4444-8444-444444444444", "name": "Build"}})
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/squads"):
			json.NewEncoder(w).Encode([]map[string]any{{"id": squadID, "name": "Build"}})
		case r.Method == "PUT" && r.URL.Path == "/api/releases/"+releaseID+"/owner":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			bodies = append(bodies, body)
			json.NewEncoder(w).Encode(map[string]any{"releases": []any{map[string]any{"id": releaseID, "owner_type": body["owner_type"], "owner_id": body["owner_id"], "owner_name": "Pieter"}}})
		default:
			t.Errorf("unexpected route %s %s (member resolution is forbidden)", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	out, err := executeReleaseTest(t, srv.URL, "assign", releaseID, "--to", "Pieter", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, agentID) || !strings.Contains(out, "owner_type") {
		t.Fatalf("owner not exposed: %s", out)
	}
	if len(bodies) != 1 || bodies[0]["owner_type"] != "agent" || bodies[0]["owner_id"] != agentID || len(bodies[0]) != 2 {
		t.Fatalf("owner-only write=%v", bodies)
	}
	if _, err := executeReleaseTest(t, srv.URL, "assign", releaseID, "--to", "Build"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguity=%v", err)
	}
	if _, err := executeReleaseTest(t, srv.URL, "assign", releaseID, "--to", "Member only"); err == nil {
		t.Fatal("resolved absent/member owner")
	}
	if _, err := executeReleaseTest(t, srv.URL, "assign", releaseID, "--unassign"); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || bodies[1]["owner_type"] != nil || bodies[1]["owner_id"] != nil {
		t.Fatalf("clear=%v", bodies)
	}
}

func TestReleaseCommentFileReplyAndBoundedRead(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Chdir(t.TempDir())
	const releaseID = "11111111-1111-4111-8111-111111111111"
	const parentID = "22222222-2222-4222-8222-222222222222"
	body := "Actual result\n\n中文 and literal \\n remain intact."
	if err := os.WriteFile(filepath.Join(".", "result.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	var posts, reads int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/releases/"+releaseID+"/comments" {
			t.Errorf("wrong native subject %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method == "POST" {
			var got map[string]any
			json.NewDecoder(r.Body).Decode(&got)
			if got["content"] != body || got["parent_id"] != parentID {
				t.Errorf("report=%v", got)
			}
			posts++
			json.NewEncoder(w).Encode(map[string]any{"id": "report", "content": body})
			return
		}
		if r.URL.Query().Get("thread") != parentID || r.URL.Query().Get("tail") != "30" {
			t.Errorf("unbounded read %s", r.URL.RawQuery)
		}
		reads++
		json.NewEncoder(w).Encode([]any{map[string]any{"id": parentID, "content": "source"}})
	}))
	defer srv.Close()
	if _, err := executeReleaseTest(t, srv.URL, "comment", "add", releaseID, "--content-file", "./result.md", "--parent", parentID, "--output", "table"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeReleaseTest(t, srv.URL, "comment", "list", releaseID, "--thread", parentID, "--tail", "30"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeReleaseTest(t, srv.URL, "comment", "list", releaseID, "--tail", "30"); err == nil {
		t.Fatal("accepted unscoped tail")
	}
	if posts != 1 || reads != 1 {
		t.Fatalf("posts/reads=%d/%d", posts, reads)
	}
}

func TestReleaseCommentRejectsForeignThreadWithoutRetry(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Chdir(t.TempDir())
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": "parent comment not found"})
	}))
	defer srv.Close()
	_, err := executeReleaseTest(t, srv.URL, "comment", "add", "11111111-1111-4111-8111-111111111111", "--content", "result", "--parent", "22222222-2222-4222-8222-222222222222")
	if err == nil || !strings.Contains(err.Error(), "parent comment not found") || calls != 1 {
		t.Fatalf("foreign thread error=%v calls=%d", err, calls)
	}
}

func TestReleaseWakeupWaitsUseNativeSubjectAndPreserveSchedules(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Chdir(t.TempDir())
	const releaseID = "11111111-1111-4111-8111-111111111111"
	const issueID = "22222222-2222-4222-8222-222222222222"
	var bodies []map[string]any
	var deletes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/releases/"+releaseID+"/wakeups") {
			t.Errorf("release wait borrowed issue subject: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			bodies = append(bodies, body)
			json.NewEncoder(w).Encode(map[string]any{"id": "wait"})
		case http.MethodGet:
			json.NewEncoder(w).Encode([]any{map[string]any{"id": "wait"}})
		case http.MethodDelete:
			deletes++
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	if _, err := executeReleaseTest(t, srv.URL, "wakeup", "create", releaseID, "--until-issue", issueID, "--until-issue-state", "ended", "--instruction", "Resume the existing workflow", "--expires-in", "2h", "--on-timeout", "wake"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeReleaseTest(t, srv.URL, "wakeup", "create", releaseID, "--kind", "at", "--after", "10m", "--instruction", "Recheck the gate"); err != nil {
		t.Fatal(err)
	}
	condition, ok := bodies[0]["condition"].(map[string]any)
	if !ok || condition["type"] != "other_issue" || condition["issue_id"] != issueID || condition["state"] != "ended" || bodies[0]["expires_in_seconds"] != float64(7200) {
		t.Fatalf("release condition=%v", bodies[0])
	}
	if bodies[1]["after_seconds"] != float64(600) {
		t.Fatalf("release schedule=%v", bodies[1])
	}
	for _, body := range bodies {
		if _, ok := body["agent_id"]; ok {
			t.Fatal("wait pinned owner instead of resolving current release owner")
		}
	}
	if _, err := executeReleaseTest(t, srv.URL, "wakeup", "list", releaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := executeReleaseTest(t, srv.URL, "wakeup", "delete", releaseID, "wait"); err != nil {
		t.Fatal(err)
	}
	if deletes != 1 {
		t.Fatalf("delete calls=%d", deletes)
	}
	if _, err := executeReleaseTest(t, srv.URL, "wakeup", "create", releaseID, "--until-status", "done"); err == nil {
		t.Fatal("release exposed issue-only condition")
	}
}

func TestReleaseCommentAttachmentsBindToReleaseNotProxyIssue(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Chdir(t.TempDir())
	const releaseID = "11111111-1111-4111-8111-111111111111"
	const attachmentID = "22222222-2222-4222-8222-222222222222"
	if err := os.WriteFile("report.txt", []byte("Delivered artifact."), 0600); err != nil {
		t.Fatal(err)
	}
	uploads, posts := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/upload-file":
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Error(err)
				http.Error(w, "multipart", 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			if r.FormValue("issue_id") != "" {
				t.Fatal("release attachment bound to proxy issue")
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			body, err := io.ReadAll(file)
			if err != nil || string(body) != "Delivered artifact." {
				t.Errorf("artifact=%q err=%v", body, err)
			}
			uploads++
			json.NewEncoder(w).Encode(map[string]any{"id": attachmentID})
		case "/api/releases/" + releaseID + "/comments":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			ids, ok := body["attachment_ids"].([]any)
			if !ok || len(ids) != 1 || ids[0] != attachmentID {
				t.Errorf("missing release binding: %v", body)
			}
			posts++
			json.NewEncoder(w).Encode(map[string]any{"id": "report"})
		default:
			t.Errorf("unexpected upload route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	if _, err := executeReleaseTest(t, srv.URL, "comment", "add", releaseID, "--content", "Real outcome.", "--attachment", "./report.txt", "--output", "table"); err != nil {
		t.Fatal(err)
	}
	if uploads != 1 || posts != 1 {
		t.Fatalf("uploads/posts=%d/%d", uploads, posts)
	}
	if _, err := executeReleaseTest(t, srv.URL, "comment", "add", releaseID, "--content", "Real outcome.", "--attachment", "./report.txt", "--attachment", "../missing.txt", "--output", "table"); err == nil {
		t.Fatal("accepted unsafe attachment")
	}
	if uploads != 1 || posts != 1 {
		t.Fatal("invalid attachment uploaded a partial set")
	}
}
