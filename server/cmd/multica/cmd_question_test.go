package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
)

const questionTestMember = "356c8712-6100-4fb2-ac42-513770588468"
const questionTestAgent = "a57c0511-1ebc-471d-a314-438ca16cc75d"
const questionTestSquad = "0b7f5d3e-2c41-4f8a-9a63-7d2e1c4b5a90"
const questionTestArchivedSquad = "9e1d2c3b-4a5f-4e6d-8c7b-1a2f3e4d5c6b"
const questionTestTask = "5d2c8e41-7b3a-4f19-9c6e-2a8b1d4f7e30"
const questionTestIssue = "c4e8a1f2-3b6d-4e9a-8f17-6d2b9c0a5e14"

func questionTestServer(t *testing.T, questionHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	t.Chdir(t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/workspaces/ws-1/members":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"user_id": questionTestMember, "name": "Brian", "email": "brian@example.com"}})
		case "/api/agents":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"id": questionTestAgent, "name": "Joeri"}})
		case "/api/squads":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"id": questionTestSquad, "name": "Finance"}, {"id": questionTestArchivedSquad, "name": "Old Finance", "archived_at": "2026-10-01T00:00:00Z"}})
		default:
			questionHandler(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	questionTestMemberLogin(t)
	return srv
}

// questionNoRequestServer fails the test on any request, so a refusal is
// proven to happen before the CLI talks to the server.
func questionNoRequestServer(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	questionTestMemberLogin(t)
}

// questionTestMemberLogin is a member's own terminal: a member token and no
// agent run in context, whatever the shell running the tests exports.
func questionTestMemberLogin(t *testing.T) {
	t.Helper()
	t.Setenv("MULTICA_TOKEN", "mul_test-token")
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv("MULTICA_DAEMON_PORT", "")
	t.Setenv(cli.TaskConfigRootEnv, "")
	t.Setenv(cli.AgentQuestionsEnv, "")
}

// questionTestRun puts the CLI in Joeri's run on an issue whose Agent
// questions setting is on, as the daemon does: a task token, the agent and
// task ids, and cli.AgentQuestionsEnv.
func questionTestRun(t *testing.T) {
	t.Helper()
	t.Setenv("MULTICA_TOKEN", "mat_test-token")
	t.Setenv("MULTICA_AGENT_ID", questionTestAgent)
	t.Setenv("MULTICA_TASK_ID", questionTestTask)
	t.Setenv(cli.AgentQuestionsEnv, "1")
}

func onlyQuestion(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	questions, _ := body["questions"].([]any)
	if len(questions) != 1 {
		t.Fatalf("expected one question in %+v", body)
	}
	question, ok := questions[0].(map[string]any)
	if !ok {
		t.Fatalf("question is not an object: %+v", questions[0])
	}
	return question
}

func TestQuestionAskMapsAllAnswerTypesAndSuggestions(t *testing.T) {
	cases := []struct {
		name       string
		answerType string
		flags      []string
		settings   string
		suggested  string
	}{
		{"text", "text", []string{"--length", "long", "--placeholder", "Details", "--suggest", "Rich text"}, `{"length":"long","placeholder":"Details"}`, `{"text":"Rich text"}`},
		{"yes no", "yes_no", []string{"--why-on-no", "--suggest", "yes"}, `{"why_on_no":true}`, `{"choice":"yes"}`},
		{"single choice", "single_choice", []string{"--option", "per line", "--option", "per total", "--allow-other", "--suggest", "per line"}, `{"options":["per line","per total"],"allow_other":true}`, `{"option":"per line"}`},
		{"multiple choice", "multiple_choice", []string{"--option", "A, with comma", "--option", "B", "--option", "C", "--min", "2", "--max", "3", "--suggest", "A, with comma", "--suggest", "B"}, `{"options":["A, with comma","B","C"],"min":2,"max":3}`, `{"options":["A, with comma","B"]}`},
		{"facts", "confirm_facts", []string{"--fact", "Tax included", "--fact", "Currency EUR"}, `{"facts":["Tax included","Currency EUR"]}`, ``},
		{"ranking", "ranking", []string{"--item", "A", "--item", "B", "--suggest", "B", "--suggest", "A"}, `{"items":["A","B"]}`, `{"order":["B","A"]}`},
		{"scale", "scale", []string{"--min", "0", "--max", "10", "--min-label", "Low", "--max-label", "High", "--suggest", "8"}, `{"min":0,"max":10,"min_label":"Low","max_label":"High"}`, `{"point":8}`},
		{"number", "number", []string{"--unit", "EUR", "--min", "-1.5", "--max", "10.5", "--suggest", "2.5"}, `{"unit":"EUR","min":-1.5,"max":10.5}`, `{"number":2.5}`},
		{"date", "date", []string{"--date-kind", "date", "--suggest", "2026-10-14"}, `{"kind":"date"}`, `{"date":"2026-10-14"}`},
		{"datetime", "date", []string{"--date-kind", "datetime", "--suggest", "2026-10-14T12:00:00Z"}, `{"kind":"datetime"}`, `{"datetime":"2026-10-14T12:00:00Z"}`},
		{"range", "date", []string{"--date-kind", "range", "--suggest", "2026-10-14T12:00:00Z/2026-10-15T12:00:00Z"}, `{"kind":"range"}`, `{"start":"2026-10-14T12:00:00Z","end":"2026-10-15T12:00:00Z"}`},
		{"member person", "person", []string{"--allow", "members", "--suggest", "brian@example.com"}, `{"allow":"members"}`, `{"type":"member","id":"` + questionTestMember + `"}`},
		{"agent person", "person", []string{"--allow", "agents", "--suggest", "Joeri"}, `{"allow":"agents"}`, `{"type":"agent","id":"` + questionTestAgent + `"}`},
		{"file", "file", []string{"--accept", "image/png", "--accept", ".pdf"}, `{"accept":["image/png",".pdf"]}`, ``},
		{"approval", "approval", []string{"--subject", "Invoice layout", "--link-title", "Preview", "--link-url", "https://example.com/invoice"}, `{"subject":"Invoice layout","link":{"title":"Preview","url":"https://example.com/invoice"}}`, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			var body map[string]any
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				posts++
				if r.Method != http.MethodPost || r.URL.Path != "/api/questions" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				_, _ = w.Write([]byte(`{"ask_id":"q-1","questions":[{"id":"q-1","ask_id":"q-1","prompt":"What is the invoice rule?"}]}`))
			})
			questionTestRun(t)
			cmd := newQuestionCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			args := []string{"ask", "--prompt", "What is the invoice rule?", "--type", tc.answerType, "--context", "Legacy invoices", "--answerer", "Brian", "--due", "2026-10-14", "--blocks-work"}
			cmd.SetArgs(append(args, tc.flags...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if posts != 1 || out.String() != "q-1\nq-1 · What is the invoice rule?\n" {
				t.Fatalf("posts=%d output=%q", posts, out.String())
			}
			if body["due_date"] != "2026-10-14" || body["blocks_work"] != true || body["answer_policy"] != "any_one" {
				t.Fatalf("lost shared fields: %+v", body)
			}
			question := onlyQuestion(t, body)
			if question["prompt"] != "What is the invoice rule?" || question["answer_type"] != tc.answerType || question["context_line"] != "Legacy invoices" {
				t.Fatalf("lost question fields: %+v", question)
			}
			if !reflect.DeepEqual(body["answerer_ids"], []any{questionTestMember}) {
				t.Fatalf("wrong member IDs: %v", body["answerer_ids"])
			}
			assertQuestionJSON(t, question["settings"], tc.settings)
			if tc.suggested == "" {
				if _, ok := question["suggested_answer"]; ok {
					t.Fatalf("unexpected suggestion: %+v", question)
				}
			} else {
				assertQuestionJSON(t, question["suggested_answer"], tc.suggested)
			}
			for _, key := range []string{"issue_id", "task_id", "answerers", "context", "type", "due", "prompt", "answer_type", "settings", "suggested_answer", "title", "squad_id"} {
				if _, ok := body[key]; ok {
					t.Fatalf("noncanonical request key %s", key)
				}
			}
		})
	}
}

func assertQuestionJSON(t *testing.T, got any, want string) {
	t.Helper()
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("got %#v want %#v", got, expected)
	}
}

func TestQuestionAskResolvesMemberNamesEmailsIDsAndDeduplicates(t *testing.T) {
	var body map[string]any
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"ask_id":"q-1","questions":[{"id":"q-1","status":"open"}]}`))
	})
	questionTestRun(t)
	cmd := newQuestionCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ask", "--prompt", "OK?", "--type", "yes_no", "--answerer", "Brian", "--answerer", "brian@example.com", "--answerer", questionTestMember, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body["answerer_ids"], []any{questionTestMember}) {
		t.Fatalf("member resolution or deduplication failed: %+v", body)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["ask_id"] != "q-1" {
		t.Fatalf("ask JSON lost ID: %s", out.String())
	}
}

func TestQuestionAskRejectsAgentBeforeQuestionPOST(t *testing.T) {
	for _, answerer := range []string{"Joeri", questionTestAgent} {
		t.Run(answerer, func(t *testing.T) {
			posts := 0
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) { posts++; w.WriteHeader(http.StatusInternalServerError) })
			questionTestRun(t)
			cmd := newQuestionCommand()
			cmd.SetArgs([]string{"ask", "--prompt", "OK?", "--type", "yes_no", "--answerer", answerer})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "Joeri") || !strings.Contains(err.Error(), "agent") {
				t.Fatalf("agent not named in error: %v", err)
			}
			if posts != 0 {
				t.Fatalf("posted an agent-answerer question %d times", posts)
			}
		})
	}
}

func TestQuestionAskInvalidSettingsAndSuggestionsNeverPost(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flags     []string
		errorTerm string
	}{
		{"outside options", []string{"--type", "single_choice", "--option", "per line", "--option", "per total", "--suggest", "per week"}, "per week"},
		{"facts suggestion", []string{"--type", "confirm_facts", "--fact", "Tax included", "--suggest", "yes"}, "suggest"},
		{"file suggestion", []string{"--type", "file", "--accept", ".pdf", "--suggest", "file"}, "suggest"},
		{"approval suggestion", []string{"--type", "approval", "--subject", "Layout", "--suggest", "approve"}, "suggest"},
		{"too few choices", []string{"--type", "multiple_choice", "--option", "A", "--option", "B", "--min", "2", "--max", "2", "--suggest", "A"}, "suggest"},
		{"bad permutation", []string{"--type", "ranking", "--item", "A", "--item", "B", "--suggest", "A", "--suggest", "A"}, "suggest"},
		{"noninteger point", []string{"--type", "scale", "--suggest", "1.5"}, "1.5"},
		{"repeated scalar", []string{"--type", "text", "--suggest", "a", "--suggest", "b"}, "repeat"},
		{"bad due", []string{"--type", "text", "--due", "2026-02-30"}, "due"},
		{"partial approval link", []string{"--type", "approval", "--subject", "Layout", "--link-url", "https://example.com"}, "link"},
		{"unknown type", []string{"--type", "unknown"}, "type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) { posts++; w.WriteHeader(http.StatusInternalServerError) })
			cmd := newQuestionCommand()
			cmd.SetArgs(append([]string{"ask", "--prompt", "Question", "--answerer", "Brian"}, tc.flags...))
			err := cmd.Execute()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.errorTerm) {
				t.Fatalf("expected %q refusal, got %v", tc.errorTerm, err)
			}
			if posts != 0 {
				t.Fatalf("invalid ask made %d question requests", posts)
			}
		})
	}
}

func TestQuestionAskPromptFileAndDefaults(t *testing.T) {
	var body map[string]any
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"ask_id":"q-1","questions":[{"id":"q-1"}]}`))
	})
	questionTestRun(t)
	const prompt = "检查发票\nKeep literal \\n and real newlines"
	if err := os.WriteFile("prompt.md", []byte(prompt+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newQuestionCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"ask", "--prompt-file", "./prompt.md", "--type", "text", "--answerer", "Brian"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	question := onlyQuestion(t, body)
	if question["prompt"] != prompt {
		t.Fatalf("file prompt changed: %q", question["prompt"])
	}
	assertQuestionJSON(t, question["settings"], `{"length":"short"}`)
	if body["blocks_work"] != true || body["answer_policy"] != "any_one" {
		t.Fatalf("default blocks_work not true or policy not any_one: %v", body)
	}
	if _, ok := body["squad_id"]; ok {
		t.Fatalf("squad sent without --squad: %v", body)
	}
	cmd = newQuestionCommand()
	cmd.SetArgs([]string{"ask", "--prompt", "inline", "--prompt-file", "./prompt.md", "--type", "text", "--answerer", "Brian"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("accepted two prompt sources")
	}
}

func TestQuestionAskQuestionsFileMapsToQuestions(t *testing.T) {
	questions := `[
  {"prompt": "Which rounding?", "context_line": "Legacy invoices", "answer_type": "single_choice", "settings": {"options": ["per line", "per total"], "allow_other": false}, "suggested_answer": {"option": "per line"}},
  {"prompt": "Is tax\nincluded?", "answer_type": "yes_no", "settings": {"why_on_no": true}},
  {"prompt": "Who signs off?", "answer_type": "person", "settings": {"allow": "members"}, "suggested_answer": {"type": "member", "id": "` + questionTestMember + `"}}
]`
	const response = `{"ask_id":"q-1","questions":[{"id":"q-1","ask_id":"q-1","ask_position":0,"prompt":"Which rounding?"},{"id":"q-2","ask_id":"q-1","ask_position":1,"prompt":"Is tax\nincluded?"},{"id":"q-3","ask_id":"q-1","ask_position":2,"prompt":"Who signs off?"}]}`
	posts := 0
	var body map[string]any
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.Method != http.MethodPost || r.URL.Path != "/api/questions" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(response))
	})
	questionTestRun(t)
	if err := os.WriteFile("questions.json", []byte(questions), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newQuestionCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ask", "--questions-file", "./questions.json", "--title", "Three things before I build the CSV writer", "--squad", "Finance", "--answerer", "Brian", "--policy", "each", "--due", "2026-10-14"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("posts=%d", posts)
	}
	assertQuestionJSON(t, body["questions"], questions)
	if body["title"] != "Three things before I build the CSV writer" || body["squad_id"] != questionTestSquad || body["answer_policy"] != "each" || body["due_date"] != "2026-10-14" || body["blocks_work"] != true {
		t.Fatalf("lost shared fields: %+v", body)
	}
	if !reflect.DeepEqual(body["answerer_ids"], []any{questionTestMember}) {
		t.Fatalf("wrong member IDs: %v", body["answerer_ids"])
	}
	if want := "q-1\nq-1 · Which rounding?\nq-2 · Is tax included?\nq-3 · Who signs off?\n"; out.String() != want {
		t.Fatalf("output %q, want %q", out.String(), want)
	}

	cmd = newQuestionCommand()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ask", "--questions-file", "./questions.json", "--title", "Three things", "--answerer", "Brian", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	assertQuestionJSON(t, result, response)
}

func TestQuestionAskSquadAndBlocksWork(t *testing.T) {
	for _, tc := range []struct {
		name       string
		flags      []string
		blocksWork bool
	}{
		{"squad without answerers, blocks work by default", []string{"--squad", "Finance"}, true},
		{"squad by ID, blocks work off", []string{"--squad", questionTestSquad, "--blocks-work=false"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/questions" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				_, _ = w.Write([]byte(`{"ask_id":"q-1","questions":[{"id":"q-1","prompt":"OK?"}]}`))
			})
			questionTestRun(t)
			cmd := newQuestionCommand()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetArgs(append([]string{"ask", "--prompt", "OK?", "--type", "yes_no"}, tc.flags...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if body["squad_id"] != questionTestSquad || body["blocks_work"] != tc.blocksWork {
				t.Fatalf("squad or blocks_work lost: %+v", body)
			}
			if !reflect.DeepEqual(body["answerer_ids"], []any{}) {
				t.Fatalf("squad-only ask sent answerers: %v", body["answerer_ids"])
			}
		})
	}
}

func TestQuestionAskArchivedOrUnknownSquadNeverPosts(t *testing.T) {
	for _, squad := range []string{"Old Finance", "Marketing"} {
		t.Run(squad, func(t *testing.T) {
			posts := 0
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) { posts++; w.WriteHeader(http.StatusInternalServerError) })
			questionTestRun(t)
			cmd := newQuestionCommand()
			cmd.SetArgs([]string{"ask", "--prompt", "OK?", "--type", "yes_no", "--squad", squad})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), squad) {
				t.Fatalf("expected a refusal naming %q, got %v", squad, err)
			}
			if posts != 0 {
				t.Fatalf("posted %d times", posts)
			}
		})
	}
}

func TestQuestionAskRefusalsBeforeAnyRequest(t *testing.T) {
	const three = `[{"prompt":"A","answer_type":"text","settings":{"length":"short"}},{"prompt":"B","answer_type":"text","settings":{"length":"short"}},{"prompt":"C","answer_type":"text","settings":{"length":"short"}}]`
	const one = `[{"prompt":"A","answer_type":"text","settings":{"length":"short"}}]`
	for _, tc := range []struct {
		name      string
		file      string
		args      []string
		errorTerm string
	}{
		{"questions file with prompt", three, []string{"--questions-file", "q.json", "--title", "T", "--prompt", "X", "--answerer", "Brian"}, "--prompt"},
		{"questions file with type", three, []string{"--questions-file", "q.json", "--title", "T", "--type", "text", "--answerer", "Brian"}, "--type"},
		{"questions file with settings flag", three, []string{"--questions-file", "q.json", "--title", "T", "--option", "A", "--answerer", "Brian"}, "--option"},
		{"questions file with suggest", three, []string{"--questions-file", "q.json", "--title", "T", "--suggest", "A", "--answerer", "Brian"}, "--suggest"},
		{"three questions without title", three, []string{"--questions-file", "q.json", "--answerer", "Brian"}, "--title"},
		{"blank title", three, []string{"--questions-file", "q.json", "--title", "  ", "--answerer", "Brian"}, "--title"},
		{"title over 200 characters", three, []string{"--questions-file", "q.json", "--title", strings.Repeat("é", 201), "--answerer", "Brian"}, "--title"},
		{"title with one question from file", one, []string{"--questions-file", "q.json", "--title", "T", "--answerer", "Brian"}, "--title"},
		{"title with one flag question", "", []string{"--prompt", "X", "--type", "text", "--title", "T", "--answerer", "Brian"}, "--title"},
		{"empty file", `[]`, []string{"--questions-file", "q.json", "--answerer", "Brian"}, "no questions"},
		{"not an array", `{"prompt":"A"}`, []string{"--questions-file", "q.json", "--answerer", "Brian"}, "json array"},
		{"unknown field", `[{"prompt":"A","type":"text","settings":{"length":"short"}}]`, []string{"--questions-file", "q.json", "--answerer", "Brian"}, "type"},
		{"invalid second question", `[{"prompt":"A","answer_type":"text","settings":{"length":"short"}},{"prompt":"B","answer_type":"single_choice","settings":{"options":["only"],"allow_other":false}}]`, []string{"--questions-file", "q.json", "--title", "T", "--answerer", "Brian"}, "question 2:"},
		{"invalid suggestion", `[{"prompt":"A","answer_type":"yes_no","settings":{"why_on_no":false},"suggested_answer":{"choice":"maybe"}}]`, []string{"--questions-file", "q.json", "--answerer", "Brian"}, "question 1:"},
		{"no answerer or squad", one, []string{"--questions-file", "q.json"}, "--squad"},
		{"unknown policy", one, []string{"--questions-file", "q.json", "--answerer", "Brian", "--policy", "all"}, "--policy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			questionNoRequestServer(t)
			if tc.file != "" {
				if err := os.WriteFile("q.json", []byte(tc.file), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newQuestionCommand()
			cmd.SetArgs(append([]string{"ask"}, tc.args...))
			err := cmd.Execute()
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.errorTerm)) {
				t.Fatalf("expected %q refusal, got %v", tc.errorTerm, err)
			}
		})
	}
}

// questionIssueTestServer serves WORK-1 as questionTestIssue and records the
// one POST /api/questions with the run headers it carried.
func questionIssueTestServer(t *testing.T, body *map[string]any, headers *http.Header) {
	t.Helper()
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/issues/WORK-1":
			_, _ = w.Write([]byte(`{"id":"` + questionTestIssue + `","identifier":"WORK-1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/questions":
			if *headers != nil {
				t.Errorf("second question POST")
			}
			*headers = r.Header.Clone()
			if err := json.NewDecoder(r.Body).Decode(body); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"ask_id":"q-1","questions":[{"id":"q-1","prompt":"Pass the plan?"}]}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func TestQuestionAskOutsideARunWithoutAnIssueOnNeverRequests(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no --on", nil},
		{"--on a project", []string{"--on", "project:1f2e3d4c"}},
		{"--on an agent", []string{"--on", "agent:Joeri"}},
		{"only --link an issue", []string{"--link", "WORK-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			questionNoRequestServer(t)
			cmd := newQuestionCommand()
			cmd.SetArgs(append([]string{"ask", "--prompt", "Pass the plan?", "--type", "yes_no", "--answerer", "Brian"}, tc.args...))
			err := cmd.Execute()
			if err == nil || err.Error() != "a question asked outside a run needs --on with an issue" {
				t.Fatalf("expected the outside-a-run refusal, got %v", err)
			}
		})
	}
}

func TestQuestionAskOutsideARunPostsTheIssueFirstAndNoRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		on   string
	}{
		{"bare key", "WORK-1"},
		{"issue: key", "issue:WORK-1"},
		{"bare ID", questionTestIssue},
		{"issue: ID", "issue:" + questionTestIssue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			var headers http.Header
			questionIssueTestServer(t, &body, &headers)
			cmd := newQuestionCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"ask", "--on", tc.on, "--prompt", "Pass the plan?", "--type", "yes_no", "--answerer", "Brian"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if headers == nil {
				t.Fatal("the question was not posted")
			}
			assertQuestionJSON(t, body["links"], `[{"type":"issue","id":"`+questionTestIssue+`"}]`)
			if headers.Get("X-Task-ID") != "" || headers.Get("X-Agent-ID") != "" {
				t.Fatalf("member ask sent run headers: %v", headers)
			}
			if !reflect.DeepEqual(body["answerer_ids"], []any{questionTestMember}) {
				t.Fatalf("wrong member IDs: %v", body["answerer_ids"])
			}
			if out.String() != "q-1\nq-1 · Pass the plan?\n" {
				t.Fatalf("output=%q", out.String())
			}
		})
	}
}

// In a run the issue rule does not apply: the server defaults the place to
// the run's issue and links the run, so the CLI sends what it always sent.
func TestQuestionAskInARunKeepsItsLinks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		links string
	}{
		{"no --on or --link", nil, ""},
		{"--link without --on", []string{"--link", "WORK-1"}, `[{"type":"issue","id":"` + questionTestIssue + `"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			var headers http.Header
			questionIssueTestServer(t, &body, &headers)
			questionTestRun(t)
			cmd := newQuestionCommand()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetArgs(append([]string{"ask", "--prompt", "Pass the plan?", "--type", "yes_no", "--answerer", "Brian"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if headers.Get("X-Task-ID") != questionTestTask || headers.Get("X-Agent-ID") != questionTestAgent {
				t.Fatalf("ask lost its run: %v", headers)
			}
			links, sent := body["links"]
			if tc.links == "" {
				if sent {
					t.Fatalf("links sent without --on or --link: %v", links)
				}
				return
			}
			assertQuestionJSON(t, links, tc.links)
		})
	}
}

func TestQuestionAnswersJSONPreservesFullAggregate(t *testing.T) {
	const response = `{"id":"q-1","prompt":"Round?","status":"answered","answers":[{"id":"a-1","answerer_user_id":"` + questionTestMember + `","value":{"option":"per line"},"display_text":"per line","note":"as the old system did","updated_at":"2026-10-12T21:04:00Z"}],"events":[{"kind":"ask_back","actor_type":"member","actor_id":"` + questionTestMember + `","details":{"text":"Which currency?"},"created_at":"2026-10-12T21:00:00Z"}],"unseen_answer_count":1,"wake_available":true}`
	gets := 0
	questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gets++
		if r.Method != http.MethodGet || r.URL.Path != "/api/questions/q-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(response))
	})
	cmd := newQuestionCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"answers", "q-1", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	assertQuestionJSON(t, result, response)
	if gets != 1 {
		t.Fatalf("expected one question GET, got %d", gets)
	}

	cmd = newQuestionCommand()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"answers", "q-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"answered", "per line", "Brian", "2026-10-12T21:04:00Z", "as the old system did", "Which currency?"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("readable answers omitted %q: %s", want, out.String())
		}
	}
}

func TestQuestionAnswersShowsTheCancel(t *testing.T) {
	for _, tc := range []struct {
		name    string
		details string
		want    string
	}{
		{"with a note", `{"note":"already decided in the PRD"}`, "- Cancelled by member:Brian: already decided in the PRD · 2026-10-15T09:30:00Z\n"},
		{"without a note", `{}`, "- Cancelled by member:Brian · 2026-10-15T09:30:00Z\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := `{"id":"q-1","prompt":"Round?","status":"cancelled","answers":[{"id":"a-1","answerer_user_id":"` + questionTestMember + `","value":{"option":"per line"},"display_text":"per line","note":"","updated_at":"2026-10-15T09:00:00Z"}],"events":[{"kind":"cancelled","actor_type":"member","actor_id":"` + questionTestMember + `","details":` + tc.details + `,"created_at":"2026-10-15T09:30:00Z"}]}`
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/questions/q-1" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				_, _ = w.Write([]byte(response))
			})
			cmd := newQuestionCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"answers", "q-1"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			got := out.String()
			if !strings.HasPrefix(got, "q-1 · cancelled\n") {
				t.Fatalf("status line does not show the question cancelled:\n%s", got)
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("cancel line missing %q:\n%s", tc.want, got)
			}
			if !strings.Contains(got, "- per line · member:Brian · 2026-10-15T09:00:00Z") {
				t.Fatalf("earlier answer is no longer readable:\n%s", got)
			}
		})
	}
}

func TestQuestionCloseAndWithdrawReturnQuestion(t *testing.T) {
	for _, action := range []string{"close", "withdraw"} {
		t.Run(action, func(t *testing.T) {
			posts := 0
			questionTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				posts++
				if r.Method != http.MethodPost || r.URL.Path != "/api/questions/q-1/"+action {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				_, _ = w.Write([]byte(`{"id":"q-1","status":"closed","answers":[]}`))
			})
			cmd := newQuestionCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{action, "q-1", "--output", "json"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			assertQuestionJSON(t, result, `{"id":"q-1","status":"closed","answers":[]}`)
			if posts != 1 {
				t.Fatalf("posts=%d", posts)
			}
		})
	}
}
