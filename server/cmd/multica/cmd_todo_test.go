package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestTodoCheckCLI(t *testing.T) {
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/todos/checks" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["member_id"] != questionTestMember || body["kind"] != "feature" || body["project_id"] != "project-id" || body["issue_id"] != "WORK-12" || body["link"] != "https://staging.example.test" {
			t.Fatalf("request %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "check-id", "details": map[string]any{"answer": "pass", "automatic": true}})
	})
	questionTestRun(t)
	cmd := newTodoCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"check", "--issue", "WORK-12", "--project", "project-id", "--answerer", "Brian", "--kind", "feature", "--link", "https://staging.example.test", "--title", "Check totals", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != "check-id" {
		t.Fatalf("output %s", out.String())
	}
}

func TestTodoAnswerCLI(t *testing.T) {
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/todos/check-id/answer" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["answer"] != "feedback" || body["note"] != "Round totals" {
			t.Fatalf("request %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "check-id"})
	})
	cmd := newTodoCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"answer", "check-id", "--answer", "feedback", "--note", "Round totals"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTodoSettingsCLI(t *testing.T) {
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/todos/check-settings" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method == "PUT" {
			var body map[string]int
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body) != 1 || body["sample_every"] != 8 {
				t.Fatalf("settings %+v", body)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"sample_every": 8})
	})
	cmd := newTodoCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"settings", "--sample-every", "8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTodoInstructionReportsCLI(t *testing.T) {
	for _, action := range []string{"instruction-note", "undo-result"} {
		t.Run(action, func(t *testing.T) {
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				path := "/api/todos/check-id/undo-result"
				if action == "instruction-note" {
					path = "/api/todos/checks/check-id/instruction-note"
				}
				if r.Method != "POST" || r.URL.Path != path {
					t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["link"] != "https://example.test/commit" || body["note"] != "Use rounded totals" {
					t.Fatalf("report: %+v", body)
				}
				if action == "instruction-note" && body["file"] != "skills/totals.md" {
					t.Fatalf("file: %+v", body)
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"id": "note-id"})
			})
			cmd := newTodoCommand()
			cmd.SetOut(&bytes.Buffer{})
			args := []string{action, "check-id", "--link", "https://example.test/commit", "--note", "Use rounded totals"}
			if action == "instruction-note" {
				args = append(args, "--file", "skills/totals.md")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
