package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const instructionSelectionKimiFixture = `#!/bin/sh
case "$*" in *--version*) echo '1.0.0'; exit 0;; esac
printf 'started' > "$SELECTION_STARTED"
while IFS= read -r line; do
  id=$(printf '%s' "$line" | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
  case "$line" in
    *'"method":"initialize"'*)
      printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1,"agentCapabilities":{}}}\n' "$id";;
    *'"method":"session/new"'*)
      printf '{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"selection-session"}}\n' "$id";;
    *'"method":"session/prompt"'*)
      printf '%s\n' "$line" > "$SELECTION_PROMPT"
      printf '%s\n' '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"selection-session","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"done"}}}}'
      printf '{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"end_turn"}}\n' "$id"
      exit 0;;
    *) if [ -n "$id" ]; then printf '{"jsonrpc":"2.0","id":%s,"result":{}}\n' "$id"; fi;;
  esac
done
`

func newInstructionSelectionTaskDaemon(t *testing.T) (*Daemon, Task, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake agent executable")
	}
	d := newTestDaemon(t)
	root := t.TempDir()
	fake := filepath.Join(root, "kimi-fixture")
	writeTestExecutable(t, fake, []byte(instructionSelectionKimiFixture))
	d.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	d.workspaces = make(map[string]*workspaceState)
	d.activeEnvRoots = make(map[string]int)
	d.runtimeIndex = map[string]Runtime{"selection-runtime": {ID: "selection-runtime", Provider: "kimi"}}
	d.cancelPollInterval = time.Hour
	d.cfg = Config{WorkspacesRoot: t.TempDir(), AgentTimeout: 5 * time.Second, ServerBaseURL: d.client.baseURL, Agents: map[string]AgentEntry{"kimi": {Path: fake}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	d.client = NewClient(srv.URL)
	d.cfg.ServerBaseURL = srv.URL
	d.runner = taskRunnerFunc(d.runTask)
	started, prompt := filepath.Join(root, "started"), filepath.Join(root, "prompt.json")
	task := Task{ID: "selection-task", WorkspaceID: "selection-workspace", RuntimeID: "selection-runtime", IssueID: "selection-issue", AgentID: "selection-agent", AuthToken: "mat_selection_test", Agent: &AgentData{ID: "selection-agent", Name: "selection-agent", Instructions: "Read first.\n\n{selection}", CustomEnv: map[string]string{"SELECTION_STARTED": started, "SELECTION_PROMPT": prompt, "KIMI_CODE_HOME": t.TempDir()}}}
	return d, task, started, prompt
}

func TestInstructionSelectionTaskFailureBeforeAgentStart(t *testing.T) {
	d, task, started, _ := newInstructionSelectionTaskDaemon(t)
	path := filepath.Join(t.TempDir(), "missing.md")
	task.Agent.InstructionSelection = []string{path}
	var failure atomic.Value
	var failCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/fail") {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode fail: %v", err)
			}
			failure.Store(body)
			failCalls.Add(1)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	d.client = NewClient(srv.URL)
	d.cfg.ServerBaseURL = srv.URL
	d.handleTask(context.Background(), task, 0)
	body, _ := failure.Load().(map[string]any)
	if failCalls.Load() != 1 || body["failure_reason"] != failureReasonInstructionSelection {
		t.Fatalf("failure calls = %d, body = %v", failCalls.Load(), body)
	}
	message := fmt.Sprint(body["error"])
	if !strings.Contains(message, path) || !strings.Contains(message, "does not exist") {
		t.Fatalf("failure error = %q", message)
	}
	if _, err := os.Stat(started); !os.IsNotExist(err) {
		t.Fatalf("agent started despite unreadable selection; marker stat = %v", err)
	}
	wrapped := &environmentSetupError{err: fmt.Errorf("wrapped: %w", &instructionSelectionError{path: path, cause: "does not exist"})}
	if got := taskRunFailureReason(wrapped); got != failureReasonInstructionSelection {
		t.Fatalf("wrapped selection failure reason = %q", got)
	}
}

func TestInstructionSelectionTaskBriefAndInlinePrompt(t *testing.T) {
	d, task, _, promptPath := newInstructionSelectionTaskDaemon(t)
	file := selectionTestFile(t, t.TempDir(), "SKILL.md", "Live instruction content one.")
	task.Agent.InstructionSelection = []string{file}
	for i, content := range []string{"Live instruction content one.", "Live instruction content two."} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		task.ID = fmt.Sprintf("selection-task-%d", i)
		result, err := d.runTask(context.Background(), task, "kimi", 0, d.logger)
		if err != nil || result.Status != "completed" {
			t.Fatalf("runTask = %+v, %v", result, err)
		}
		brief, err := os.ReadFile(filepath.Join(result.WorkDir, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		want := "Read first.\n\n<selection>\n<file path=\"" + file + "\">\n" + content + "\n</file>\n</selection>"
		if !strings.Contains(string(brief), want) || strings.Contains(string(brief), "{selection}") {
			t.Fatalf("brief lacks filled instructions: %s", brief)
		}
		raw, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatal(err)
		}
		var request struct {
			Params struct {
				Prompt []struct {
					Text string `json:"text"`
				} `json:"prompt"`
			} `json:"params"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Fatal(err)
		}
		if len(request.Params.Prompt) != 1 || !strings.Contains(request.Params.Prompt[0].Text, want) || strings.Contains(request.Params.Prompt[0].Text, "{selection}") {
			t.Fatalf("inline SystemPrompt lacks filled instructions: %s", raw)
		}
	}
}
