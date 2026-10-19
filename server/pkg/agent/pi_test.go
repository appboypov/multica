package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// omp 18.10 refuses `--session` on an empty file ("the session file holds no
// entries"), so every new omp task failed at start. An omp session file that
// is missing or empty must start with a session header; pi keeps its empty
// file, and a file with entries is never rewritten.
func TestEnsurePiSessionFile(t *testing.T) {
	dir := t.TempDir()

	piPath := filepath.Join(dir, "pi", "new.jsonl")
	if err := ensurePiSessionFile(piPath, false, dir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(piPath); len(data) != 0 {
		t.Errorf("pi session file = %q, want empty", data)
	}

	ompPath := filepath.Join(dir, "omp", "new.jsonl")
	if err := ensurePiSessionFile(ompPath, true, "/work/dir"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ompPath)
	var header map[string]any
	if err := json.Unmarshal(data, &header); err != nil {
		t.Fatalf("omp session file %q is not one JSON line: %v", data, err)
	}
	if header["type"] != "session" || header["cwd"] != "/work/dir" || header["id"] == "" {
		t.Errorf("omp header = %v, want a session header for /work/dir", header)
	}

	resumed := filepath.Join(dir, "omp", "resumed.jsonl")
	const history = "{\"type\":\"session\",\"id\":\"x\"}\n{\"type\":\"message\"}\n"
	if err := os.WriteFile(resumed, []byte(history), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensurePiSessionFile(resumed, true, dir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(resumed); string(data) != history {
		t.Errorf("resumed session file = %q, want it untouched", data)
	}
}

func TestBuildPiArgsNoToolAllowlist(t *testing.T) {
	// Extension tools registered via Pi's registerTool() must not be
	// filtered out by a hardcoded --tools allowlist. Omitting --tools
	// lets Pi use its full tool registry. See #2379.
	args := buildPiArgs("/tmp/session.jsonl", ExecOptions{}, slog.Default(), false)
	for i, arg := range args {
		if arg == "--tools" {
			t.Errorf("buildPiArgs emits --tools %q; should not restrict tool registry (see #2379)", args[i+1])
		}
	}
}

func TestBuildPiArgsBasicFlags(t *testing.T) {
	args := buildPiArgs("/tmp/s.jsonl", ExecOptions{
		Model:         "anthropic/claude-sonnet-4-20250514",
		ThinkingLevel: "high",
	}, slog.Default(), false)

	joined := strings.Join(args, " ")
	for _, want := range []string{"-p", "--mode json", "--session /tmp/s.jsonl", "--model anthropic/claude-sonnet-4-20250514", "--thinking high"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in args, got: %v", want, args)
		}
	}

	for _, arg := range args {
		if arg == "hello world" {
			t.Fatalf("prompt leaked into argv: %v", args)
		}
	}
}

// Pi reads the per-task AGENTS.md the daemon writes into the workdir, so the
// daemon never populates SystemPrompt for it (providerNeedsInlineSystemPrompt).
// Forwarding it anyway would duplicate the whole runtime brief on every turn.
func TestBuildPiArgsIgnoresSystemPrompt(t *testing.T) {
	args := buildPiArgs("/tmp/s.jsonl", ExecOptions{
		SystemPrompt: "the entire multica runtime brief",
	}, slog.Default(), false)

	for _, a := range args {
		if a == "--append-system-prompt" {
			t.Fatalf("unexpected --append-system-prompt in args: %v", args)
		}
		if a == "the entire multica runtime brief" {
			t.Fatalf("SystemPrompt leaked into args: %v", args)
		}
	}
}

func TestBuildPiArgsCustomArgsAppended(t *testing.T) {
	// Users can still restrict tools via custom_args if desired.
	args := buildPiArgs("/tmp/s.jsonl", ExecOptions{
		CustomArgs: []string{"--tools", "read,bash"},
	}, slog.Default(), false)

	found := false
	for i, arg := range args {
		if arg == "--tools" && i+1 < len(args) && args[i+1] == "read,bash" {
			found = true
		}
	}
	if !found {
		t.Errorf("custom --tools should pass through via custom_args, got: %v", args)
	}
}

func TestBuildPiArgsOmpSkillSettings(t *testing.T) {
	t.Parallel()

	path := "/tmp/task env/omp-runtime-skill-settings.yml"
	args := buildPiArgs("", ExecOptions{
		CustomArgs:      []string{"--config", "/tmp/user.yml", "--tools", "read,bash"},
		OmpSettingsPath: path,
	}, slog.Default(), true)
	if !slices.Equal(args[len(args)-2:], []string{"--config", path}) {
		t.Fatalf("task skill settings must override custom config: %q", args)
	}
	args = buildPiArgs("", ExecOptions{}, slog.Default(), true)
	if slices.Contains(args, "--config") {
		t.Fatalf("args without omp settings unexpectedly contain --config: %q", args)
	}
}

func TestBuildPiArgsFiltersCustomInputButKeepsOptionValues(t *testing.T) {
	t.Parallel()

	args := buildPiArgs("/tmp/s.jsonl", ExecOptions{
		CustomArgs: []string{
			"--tools", "read,bash",
			"positional-input",
			"@prompt.md",
			"--verbose",
			"after-boolean",
			"--extension-option", "extension-value",
			"--thinking", "low",
			"--thinking=medium",
			"--offline",
			"trailing-input",
		},
	}, slog.Default(), false)

	joined := strings.Join(args, "\x00")
	for _, unwanted := range []string{"positional-input", "@prompt.md", "after-boolean", "trailing-input"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("custom input %q should be filtered, got %v", unwanted, args)
		}
	}
	for _, pair := range [][2]string{
		{"--tools", "read,bash"},
		{"--extension-option", "extension-value"},
	} {
		found := false
		for i := 0; i+1 < len(args); i++ {
			if args[i] == pair[0] && args[i+1] == pair[1] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("option/value %q %q missing from %v", pair[0], pair[1], args)
		}
	}
	for _, arg := range args {
		if arg == "--thinking" || strings.HasPrefix(arg, "--thinking=") || arg == "low" || arg == "medium" {
			t.Errorf("custom --thinking must be owned by thinking_level and filtered, got %v", args)
		}
	}
}

func TestBuildPiArgsThinkingLevelOverridesCustomArgs(t *testing.T) {
	t.Parallel()

	args := buildPiArgs("/tmp/s.jsonl", ExecOptions{
		ThinkingLevel: "max",
		CustomArgs:    []string{"--thinking", "low", "--thinking=medium"},
	}, slog.Default(), false)

	count := 0
	for i, arg := range args {
		if arg == "--thinking" {
			count++
			if i+1 >= len(args) || args[i+1] != "max" {
				t.Fatalf("selected thinking level missing from args: %v", args)
			}
		}
		if strings.HasPrefix(arg, "--thinking=") {
			t.Fatalf("custom inline thinking flag leaked through: %v", args)
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one daemon-owned --thinking flag, got %d in %v", count, args)
	}
}

func TestPiExecuteRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()

	backend, err := New("pi", Config{ExecutablePath: "/does/not/need/to/exist", Logger: slog.Default()})
	if err != nil {
		t.Fatalf("New(pi): %v", err)
	}
	if _, err := backend.Execute(t.Context(), " \n\t ", ExecOptions{}); err == nil || !strings.Contains(err.Error(), "prompt must not be empty") {
		t.Fatalf("Execute error = %v, want empty-prompt error", err)
	}
}

func TestPiExecuteRejectsConcurrentSessionWriter(t *testing.T) {
	t.Parallel()

	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("create session file: %v", err)
	}
	claim, locked, err := tryLockPiSessionFile(sessionPath)
	if err != nil {
		t.Fatalf("lock session file: %v", err)
	}
	if !locked {
		t.Fatal("first session-file lock was unexpectedly busy")
	}
	defer releasePiSessionFileLock(claim)

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	backend, err := New("pi", Config{ExecutablePath: executable, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("New(pi): %v", err)
	}
	session, err := backend.Execute(t.Context(), "follow-up", ExecOptions{ResumeSessionID: sessionPath})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for range session.Messages {
	}
	result, ok := <-session.Result
	if !ok {
		t.Fatal("result channel closed without a value")
	}
	if result.Status != "failed" || !result.ResumeRejectedTransient {
		t.Fatalf("result = %+v, want failed ResumeRejectedTransient", result)
	}
	if result.ResumeRejected {
		t.Fatal("a busy session is still healthy and must not be permanently rejected")
	}
	if result.SessionID != "" {
		t.Fatalf("SessionID = %q, want empty so the live transcript is not republished", result.SessionID)
	}
	if !strings.Contains(result.Error, "already in use") {
		t.Fatalf("error = %q, want session-in-use diagnostic", result.Error)
	}
}

// TestPiExecuteAttachesStdinPipe verifies that the Pi backend spawns the child
// with an explicit stdin pipe, writes the task prompt, and closes it. Closing
// delivers both the end-of-prompt signal and the EOF that keeps Pi from
// blocking under systemd (#2188).
//
// The probe is structural rather than behavioral: a shell script in
// place of `pi` inspects /proc/self/fd/0, drains it to EOF, and only emits a
// valid event stream when both the pipe type and prompt are correct.
func TestPiExecuteAttachesStdinPipe(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		// /proc/self/fd/0 is Linux-specific; skipping elsewhere keeps
		// the assertion portable without losing CI coverage.
		t.Skip("stdin fd inspection relies on /proc/self/fd/0")
	}

	fakePath := filepath.Join(t.TempDir(), "pi")
	script := "#!/bin/sh\n" +
		"kind=$(stat -c '%F' -L /proc/self/fd/0 2>/dev/null || echo unknown)\n" +
		"payload=$(cat)\n" +
		"case \"$kind\" in\n" +
		"  fifo|*pipe*)\n" +
		"    if [ \"$payload\" = 'prompt-over-stdin' ]; then\n" +
		"      printf '%s\\n' '{\"type\":\"agent_start\"}'\n" +
		"      printf '%s\\n' '{\"type\":\"turn_end\",\"message\":{\"role\":\"assistant\",\"model\":\"test\",\"usage\":{\"input\":1,\"output\":1,\"cacheRead\":0,\"cacheWrite\":0,\"totalTokens\":2}}}'\n" +
		"      exit 0\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n" +
		"printf 'stdin was %s with payload %s; expected fifo and prompt\\n' \"$kind\" \"$payload\" >&2\n" +
		"exit 1\n"
	writeTestExecutable(t, fakePath, []byte(script))

	backend, err := New("pi", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new pi backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	session, err := backend.Execute(ctx, "prompt-over-stdin", ExecOptions{
		Timeout:         5 * time.Second,
		ResumeSessionID: sessionPath,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()

	select {
	case result, ok := <-session.Result:
		if !ok {
			t.Fatal("result channel closed without a value")
		}
		if result.Status != "completed" {
			t.Fatalf("expected status=completed (stdin attached as fifo), got %q (error=%q)", result.Status, result.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}
	claim, locked, err := tryLockPiSessionFile(sessionPath)
	if err != nil {
		t.Fatalf("lock completed session: %v", err)
	}
	if !locked {
		t.Fatal("Pi backend returned its result before releasing the session-file lock")
	}
	releasePiSessionFileLock(claim)
}

// piEventStreamScript builds a sh script that prints each JSON event on
// its own stdout line. Fixtures must not contain single quotes.
func piEventStreamScript(events []string) string {
	return piEventStreamScriptWithExit(events, 0)
}

// piEventStreamScriptWithExit is piEventStreamScript plus an explicit
// process exit code. Real Pi (and pi-print-clean-exit) exits 1 after a
// turn whose last assistant stopReason is "error".
func piEventStreamScriptWithExit(events []string, exitCode int) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	// Real Pi reads the piped prompt to EOF before emitting events, so the fake
	// must drain stdin too. A fake that exits without reading closes the read end
	// while the backend is still writing the prompt, and the resulting EPIPE is
	// reported as "pi prompt write failed" — a load-dependent flake that has
	// nothing to do with what these tests assert.
	b.WriteString("cat > /dev/null\n")
	for _, e := range events {
		b.WriteString("printf '%s\\n' '")
		b.WriteString(e)
		b.WriteString("'\n")
	}
	if exitCode != 0 {
		b.WriteString(fmt.Sprintf("exit %d\n", exitCode))
	}
	return b.String()
}

func newPiTestBackend(t *testing.T, script string, turnErrorGrace time.Duration) *piBackend {
	t.Helper()
	fakePath := filepath.Join(t.TempDir(), "pi")
	writeTestExecutable(t, fakePath, []byte(script))

	backend, err := New("pi", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new pi backend: %v", err)
	}
	pi, ok := backend.(*piBackend)
	if !ok {
		t.Fatalf("New(pi) returned %T, want *piBackend", backend)
	}
	pi.turnErrorGrace = turnErrorGrace
	return pi
}

func waitPiResult(t *testing.T, session *Session, timeout time.Duration) Result {
	t.Helper()
	select {
	case result, ok := <-session.Result:
		if !ok {
			t.Fatal("result channel closed without a value")
		}
		return result
	case <-time.After(timeout):
		t.Fatal("timeout waiting for Pi result")
		return Result{}
	}
}

func TestPiExecutePreservesTurnErrorWhenCancelled(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	const providerError = "OpenAI API error (413): Failed to buffer the request body: length limit exceeded"
	script := piEventStreamScript([]string{
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"turn_end","message":{"role":"assistant","model":"test","stopReason":"error","errorMessage":"` + providerError + `"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"post-error activity"}}`,
	}) + "exec sleep 300\n"
	backend := newPiTestBackend(t, script, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{
		ResumeSessionID: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for msg := range session.Messages {
		if msg.Type == MessageThinking && msg.Content == "post-error activity" {
			cancel()
			break
		}
	}

	result := waitPiResult(t, session, 5*time.Second)
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed (error=%q)", result.Status, result.Error)
	}
	if result.Error != providerError {
		t.Fatalf("error = %q, want original provider error %q", result.Error, providerError)
	}
}

func TestPiExecuteCancellationWithoutTurnErrorKeepsAbortedResult(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	script := piEventStreamScript([]string{`{"type":"agent_start"}`}) + "exec sleep 300\n"
	backend := newPiTestBackend(t, script, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{
		ResumeSessionID: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for msg := range session.Messages {
		if msg.Type == MessageStatus {
			cancel()
			break
		}
	}

	result := waitPiResult(t, session, 5*time.Second)
	if result.Status != "aborted" || result.Error != "execution cancelled" {
		t.Fatalf("result = %+v, want the existing no-error cancellation result", result)
	}
}

func TestPiExecuteEndsSilentTurnErrorAfterGraceOnce(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	const providerError = "OpenAI API error (413): request body too large"
	const grace = 80 * time.Millisecond
	script := piEventStreamScript([]string{
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"turn_end","message":{"role":"assistant","model":"test","stopReason":"error","errorMessage":"` + providerError + `"}}`,
		`{"type":"agent_end","messages":[],"willRetry":false}`,
	}) + "exec sleep 300\n"
	backend := newPiTestBackend(t, script, grace)

	started := time.Now()
	session, err := backend.Execute(context.Background(), "prompt-ignored", ExecOptions{
		Timeout:         15 * time.Second,
		ResumeSessionID: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	result := waitPiResult(t, session, 20*time.Second)
	if result.Status != "failed" || result.Error != providerError {
		t.Fatalf("result = %+v, want one failed result with the provider error", result)
	}
	for msg := range session.Messages {
		if msg.Type == MessageError {
			t.Fatalf("turn-error grace emitted an error message and would refresh the daemon watchdog: %+v", msg)
		}
	}
	if elapsed := time.Since(started); elapsed < grace/2 || elapsed > 10*time.Second {
		t.Fatalf("error grace ended after %s, want approximately %s", elapsed, grace)
	}
	if _, ok := <-session.Result; ok {
		t.Fatal("result channel produced more than one terminal result")
	}
}

func TestPiExecuteTurnErrorActivityAndRetryRecoveryCancelTimer(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	const grace = 100 * time.Millisecond
	script := "#!/bin/sh\n" +
		"cat > /dev/null\n" +
		`printf '%s\n' '{"type":"agent_start"}'` + "\n" +
		`printf '%s\n' '{"type":"turn_start"}'` + "\n" +
		`printf '%s\n' '{"type":"turn_end","message":{"role":"assistant","model":"test","stopReason":"error","errorMessage":"temporary provider error"}}'` + "\n" +
		"sleep 0.06\n" +
		`printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"retrying"}}'` + "\n" +
		"sleep 0.06\n" +
		`printf '%s\n' '{"type":"auto_retry_start","attempt":1,"maxAttempts":3,"delayMs":1}'` + "\n" +
		"sleep 0.15\n" +
		`printf '%s\n' '{"type":"turn_start"}'` + "\n" +
		`printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"recovered"}}'` + "\n" +
		`printf '%s\n' '{"type":"turn_end","message":{"role":"assistant","model":"test"}}'` + "\n"
	backend := newPiTestBackend(t, script, grace)

	session, err := backend.Execute(context.Background(), "prompt-ignored", ExecOptions{
		Timeout:         15 * time.Second,
		ResumeSessionID: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	result := waitPiResult(t, session, 20*time.Second)
	if result.Status != "completed" || result.Output != "recovered" || result.Error != "" {
		t.Fatalf("result = %+v, want successful recovered turn", result)
	}
}

func TestPiExecuteTurnErrorGraceWaitsForInFlightTool(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	const grace = 60 * time.Millisecond
	script := "#!/bin/sh\n" +
		"cat > /dev/null\n" +
		`printf '%s\n' '{"type":"turn_start"}'` + "\n" +
		`printf '%s\n' '{"type":"tool_execution_start","toolCallId":"call-1","toolName":"bash","args":{}}'` + "\n" +
		`printf '%s\n' '{"type":"turn_end","message":{"role":"assistant","model":"test","stopReason":"error","errorMessage":"temporary provider error"}}'` + "\n" +
		"sleep 0.15\n" +
		`printf '%s\n' '{"type":"tool_execution_end","toolCallId":"call-1","toolName":"bash","result":"ok"}'` + "\n" +
		`printf '%s\n' '{"type":"turn_start"}'` + "\n" +
		`printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"done"}}'` + "\n" +
		`printf '%s\n' '{"type":"turn_end","message":{"role":"assistant","model":"test"}}'` + "\n"
	backend := newPiTestBackend(t, script, grace)

	session, err := backend.Execute(context.Background(), "prompt-ignored", ExecOptions{
		Timeout:         15 * time.Second,
		ResumeSessionID: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	result := waitPiResult(t, session, 20*time.Second)
	if result.Status != "completed" || result.Output != "done" {
		t.Fatalf("result = %+v, want tool completion and recovered turn", result)
	}
}

func TestPiExecuteNormalSilenceDoesNotArmTurnErrorGrace(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	const grace = 50 * time.Millisecond
	script := "#!/bin/sh\n" +
		"cat > /dev/null\n" +
		`printf '%s\n' '{"type":"agent_start"}'` + "\n" +
		"sleep 0.15\n" +
		`printf '%s\n' '{"type":"turn_start"}'` + "\n" +
		`printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"healthy"}}'` + "\n" +
		`printf '%s\n' '{"type":"turn_end","message":{"role":"assistant","model":"test"}}'` + "\n"
	backend := newPiTestBackend(t, script, grace)

	session, err := backend.Execute(context.Background(), "prompt-ignored", ExecOptions{
		Timeout:         15 * time.Second,
		ResumeSessionID: filepath.Join(t.TempDir(), "session.jsonl"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	result := waitPiResult(t, session, 20*time.Second)
	if result.Status != "completed" || result.Output != "healthy" {
		t.Fatalf("result = %+v, want normal silent run to complete", result)
	}
}

// TestPiExecuteRetainsOnlyLastTurnOutput verifies turn_start resets the
// output buffer so Result.Output keeps only the final turn's text.
func TestPiExecuteRetainsOnlyLastTurnOutput(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	events := []string{
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"intermediate"}}`,
		`{"type":"tool_execution_start","toolCallId":"call_1","toolName":"bash","args":{"command":"echo hi"}}`,
		`{"type":"tool_execution_end","toolCallId":"call_1","toolName":"bash","result":{"content":[{"type":"text","text":"hi"}]},"isError":false}`,
		`{"type":"turn_end","message":{"role":"assistant","model":"test","usage":{"input":1,"output":1}}}`,
		`{"type":"turn_start"}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"final"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":" "}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"answer"}}`,
		`{"type":"turn_end","message":{"role":"assistant","model":"test","usage":{"input":2,"output":2}}}`,
	}
	fakePath := filepath.Join(t.TempDir(), "pi")
	writeTestExecutable(t, fakePath, []byte(piEventStreamScript(events)))

	backend, err := New("pi", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new pi backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()

	select {
	case result := <-session.Result:
		if result.Status != "completed" {
			t.Fatalf("expected status=completed, got %q (error=%q)", result.Status, result.Error)
		}
		if result.Output != "final answer" {
			t.Fatalf("Output: got %q, want %q", result.Output, "final answer")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestPiExecuteFailsOnUnretriedTurnError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	// Pi ends the turn with stopReason=error and declines to retry (401, 429
	// and friends). It emits no `error` event and no auto_retry_end, and exits
	// 0, so the terminal state lives only on turn_end.
	events := []string{
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"turn_end","message":{"role":"assistant","content":[],"model":"test","usage":{"input":0,"output":0},"stopReason":"error","errorMessage":"OpenAI API error (401): invalid token"}}`,
		`{"type":"agent_end","messages":[],"willRetry":false}`,
	}
	fakePath := filepath.Join(t.TempDir(), "pi")
	writeTestExecutable(t, fakePath, []byte(piEventStreamScript(events)))

	backend, err := New("pi", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new pi backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()

	select {
	case result := <-session.Result:
		if result.Status != "failed" {
			t.Fatalf("expected status=failed, got %q (error=%q)", result.Status, result.Error)
		}
		if !strings.Contains(result.Error, "invalid token") {
			t.Fatalf("Error: got %q, want it to carry the provider message", result.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestPiExecuteSucceedsWhenRetryFollowsTurnError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	// The same stopReason=error precedes an automatic retry. The retry
	// succeeds, so the run must not inherit the first turn's failure.
	events := []string{
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"turn_end","message":{"role":"assistant","content":[],"model":"test","usage":{"input":0,"output":0},"stopReason":"error","errorMessage":"OpenAI API error (503): no available channel"}}`,
		`{"type":"agent_end","messages":[],"willRetry":true}`,
		`{"type":"auto_retry_start","attempt":1,"maxAttempts":3,"delayMs":1}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"recovered"}}`,
		`{"type":"turn_end","message":{"role":"assistant","model":"test","usage":{"input":2,"output":2}}}`,
	}
	fakePath := filepath.Join(t.TempDir(), "pi")
	writeTestExecutable(t, fakePath, []byte(piEventStreamScript(events)))

	backend, err := New("pi", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new pi backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()

	select {
	case result := <-session.Result:
		if result.Status != "completed" {
			t.Fatalf("expected status=completed, got %q (error=%q)", result.Status, result.Error)
		}
		if result.Output != "recovered" {
			t.Fatalf("Output: got %q, want %q", result.Output, "recovered")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestPiExecuteKeepsTurnErrorWhenProcessExitsNonZero(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	// Real Pi (and pi-print-clean-exit) exits 1 after stopReason=error.
	// Wait() used to win the completed-branch and drop lastTurnError, so
	// the classifier only saw "exit status 1" (non-retryable
	// process_failure). The provider message must survive so
	// "Connection error." / "Request timed out." classify as
	// provider_network.
	events := []string{
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"turn_end","message":{"role":"assistant","content":[],"model":"test","usage":{"input":0,"output":0},"stopReason":"error","errorMessage":"Connection error."}}`,
		`{"type":"agent_end","messages":[],"willRetry":false}`,
	}
	fakePath := filepath.Join(t.TempDir(), "pi")
	writeTestExecutable(t, fakePath, []byte(piEventStreamScriptWithExit(events, 1)))

	backend, err := New("pi", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new pi backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()

	select {
	case result := <-session.Result:
		if result.Status != "failed" {
			t.Fatalf("expected status=failed, got %q (error=%q)", result.Status, result.Error)
		}
		if !strings.Contains(result.Error, "Connection error.") {
			t.Fatalf("Error: got %q, want it to carry the provider message", result.Error)
		}
		if !strings.Contains(result.Error, "exited with error") {
			t.Fatalf("Error: got %q, want the process exit still visible as a suffix", result.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

func TestStripPiToolCallMarkup(t *testing.T) {
	tests := map[string]string{
		`before call:bash{command:<|"|>cd repo/path && ls -F<|"|>}<tool_call|> after`:                           "before  after",
		`before call:read{path:<|"|>repo/path/roles/example/verify.yml<|"|>} after`:                             "before  after",
		`before response:bash{command:<|"|>multica issue comment list issue-id --all --output json<|"|>} after`: "before  after",
		`before call:bash{command:<|"|>printf '{"key":"value"}'<|"|>} after`:                                    "before  after",
		`before <|turn>model after`: "before  after",
	}
	for in, want := range tests {
		got := stripPiToolCallMarkup(in)
		if got != want {
			t.Fatalf("unexpected stripped text: %q, want %q", got, want)
		}
	}
}

func TestDrainPiTextBufferSplitToolCall(t *testing.T) {
	chunks := []string{
		"before ca",
		`ll:bash{command:<|"|>ls -R repo/path`,
		`/roles/example<|"|>}`,
		" after",
	}
	var buf strings.Builder
	var got strings.Builder
	for _, chunk := range chunks {
		got.WriteString(drainPiTextBuffer(&buf, chunk))
	}
	got.WriteString(flushPiTextBuffer(&buf))
	if got.String() != "before  after" {
		t.Fatalf("unexpected streamed text: %q", got.String())
	}
}

func TestDrainPiTextBufferSplitControlToken(t *testing.T) {
	chunks := []string{"before <|tu", "rn>model after"}
	var buf strings.Builder
	var got strings.Builder
	for _, chunk := range chunks {
		got.WriteString(drainPiTextBuffer(&buf, chunk))
	}
	got.WriteString(flushPiTextBuffer(&buf))
	if got.String() != "before  after" {
		t.Fatalf("unexpected streamed text: %q", got.String())
	}
}

func TestFlushPiTextBufferKeepsUnmatchedToolPrefixes(t *testing.T) {
	tests := []string{
		"plain response: see below",
		"plain call: see below",
		`plain call:bash{command:<|"|>unterminated`,
	}
	for _, want := range tests {
		var buf strings.Builder
		got := drainPiTextBuffer(&buf, want)
		got += flushPiTextBuffer(&buf)
		if got != want {
			t.Fatalf("unexpected flushed text: %q, want %q", got, want)
		}
	}
}

// TestBuildPiArgsKeepsSlashShapedModelIDIntact is the GH #7300 regression.
// A gateway-style provider registers model ids that themselves contain a
// slash (`claude/claude-opus-5` under provider `multica-anthropic`). Splitting
// on that slash to fill --provider produced `--provider claude --model
// claude-opus-5`, and pi answers an unknown --provider with a hard
// `Unknown provider "claude"` before it ever looks at --model. The whole
// selector must reach --model instead, where pi's resolver matches it as a
// raw model id.
func TestBuildPiArgsKeepsSlashShapedModelIDIntact(t *testing.T) {
	args := buildPiArgs("/tmp/s.jsonl", ExecOptions{Model: "claude/claude-opus-5"}, slog.Default(), false)

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--model claude/claude-opus-5") {
		t.Errorf("model id was not passed through intact: %v", args)
	}
	for _, arg := range args {
		if arg == "--provider" {
			t.Errorf("buildPiArgs synthesized --provider from a model id: %v", args)
		}
	}
}

// A fully-qualified selector is passed through the same way: pi infers the
// provider from the prefix itself, so the daemon never has to take it apart.
func TestBuildPiArgsPassesQualifiedSelectorWhole(t *testing.T) {
	args := buildPiArgs("/tmp/s.jsonl", ExecOptions{Model: "multica-anthropic/claude/claude-opus-5"}, slog.Default(), false)

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--model multica-anthropic/claude/claude-opus-5") {
		t.Errorf("qualified selector was not passed through intact: %v", args)
	}
	if strings.Contains(joined, "--provider") {
		t.Errorf("buildPiArgs synthesized --provider: %v", args)
	}
}

// A bare model id stays bare, and an empty model still omits --model entirely
// so pi resolves its own default.
func TestBuildPiArgsBareAndEmptyModels(t *testing.T) {
	bare := buildPiArgs("", ExecOptions{Model: "  claude-opus-5  "}, slog.Default(), false)
	if joined := strings.Join(bare, " "); !strings.Contains(joined, "--model claude-opus-5") {
		t.Errorf("bare model not forwarded (or not trimmed): %v", bare)
	}

	empty := buildPiArgs("", ExecOptions{Model: "   "}, slog.Default(), false)
	for _, arg := range empty {
		if arg == "--model" {
			t.Errorf("blank model should omit --model so pi picks its default: %v", empty)
		}
	}
}

func TestPiAndOmpExecuteEventParity(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}
	events := []string{
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":{"role":"assistant"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"considering"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello "}}`,
		`{"type":"tool_execution_start","toolCallId":"call-1","toolName":"read","args":{"path":"file.txt"}}`,
		`{"type":"tool_execution_update","toolCallId":"call-1"}`,
		`{"type":"tool_execution_end","toolCallId":"call-1","result":"contents"}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"world"}}`,
		`{"type":"message_end","message":{"role":"assistant"}}`,
		`{"type":"turn_end","message":{"role":"assistant","model":"fixture","usage":{"input":10,"output":5,"cacheRead":3,"cacheWrite":2},"stopReason":"stop"}}`,
		`{"type":"agent_end","messages":[]}`,
	}
	const prompt = "a \"quoted\" prompt\nwith a second line and 👀"
	var printMessages []Message
	var printResult Result
	for _, provider := range []string{"pi", "omp"} {
		t.Run(provider, func(t *testing.T) {
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "input")
			argsPath := filepath.Join(dir, "args")
			eofPath := filepath.Join(dir, "eof")
			fakePath := filepath.Join(dir, provider)
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_PATH\"\n"
			if provider == "pi" {
				script += "cat > \"$INPUT_PATH\"\n"
			} else {
				script += "IFS= read -r mode || exit 11\nIFS= read -r prompt || exit 12\n" +
					"printf '%s\\n' \"$mode\" \"$prompt\" > \"$INPUT_PATH\"\n" +
					"printf '%s\\n' '{\"type\":\"ready\"}' '{\"type\":\"response\",\"command\":\"prompt\",\"success\":true}'\n"
			}
			for _, event := range events {
				script += "printf '%s\\n' '" + event + "'\n"
			}
			if provider == "omp" {
				script += "printf '%s\\n' '{\"type\":\"prompt_result\",\"status\":\"completed\",\"sessionSettled\":true}' '{\"type\":\"session_settled\"}'\n" +
					"if IFS= read -r extra; then exit 13; fi\n"
			}
			script += "printf 'eof' > \"$EOF_PATH\"\n"
			writeTestExecutable(t, fakePath, []byte(script))
			backend, err := ResolveBackend(provider, Config{
				ExecutablePath: fakePath, Logger: slog.Default(),
				Env: map[string]string{"INPUT_PATH": inputPath, "ARGS_PATH": argsPath, "EOF_PATH": eofPath},
			})
			if err != nil {
				t.Fatal(err)
			}
			sessionPath := filepath.Join(dir, "session.jsonl")
			session, err := backend.Execute(t.Context(), prompt, ExecOptions{
				ResumeSessionID: sessionPath, Timeout: 5 * time.Second,
				Model: "gateway/fixture", ThinkingLevel: "high",
			})
			if err != nil {
				t.Fatal(err)
			}
			if session.Supplement != nil || session.SupplementReady != nil {
				t.Fatal("session without negotiated supplements exposes input")
			}
			var messages []Message
			for message := range session.Messages {
				messages = append(messages, message)
			}
			result := waitPiResult(t, session, time.Second)
			if result.Status != "completed" || result.Error != "" || result.Output != "hello world" {
				t.Fatalf("unexpected result: %+v", result)
			}
			if result.SessionID != sessionPath {
				t.Fatalf("session id = %q, want %q", result.SessionID, sessionPath)
			}
			if eof, err := os.ReadFile(eofPath); err != nil || string(eof) != "eof" {
				t.Fatalf("child did not observe stdin EOF: %q, %v", eof, err)
			}
			args, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			wantArgs := []string{"-p", "--mode", "json"}
			if provider == "omp" {
				wantArgs = []string{"--mode", "rpc", "--no-ui"}
			}
			wantArgs = append(wantArgs, "--session", sessionPath, "--model", "gateway/fixture", "--thinking", "high")
			if string(args) != strings.Join(wantArgs, "\n")+"\n" {
				t.Fatalf("argv = %q, want %v", args, wantArgs)
			}
			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
			// Compare only provider-independent result fields.
			result.DurationMs, result.SessionID = 0, ""
			if provider == "pi" {
				if string(input) != prompt {
					t.Fatalf("print-mode prompt = %q", input)
				}
				printMessages, printResult = messages, result
				if len(messages) != 6 || messages[0].Type != MessageStatus ||
					messages[1].Type != MessageThinking || messages[3].Type != MessageToolUse ||
					messages[4].Type != MessageToolResult {
					t.Fatalf("missing print-mode mappings: %+v", messages)
				}
				if got := result.Usage["fixture"]; got.InputTokens != 10 || got.OutputTokens != 5 ||
					got.CacheReadTokens != 3 || got.CacheWriteTokens != 2 {
					t.Fatalf("usage = %+v", got)
				}
				return
			}
			lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("initial input has %d physical lines: %q", len(lines), input)
			}
			var mode, initial map[string]string
			if err := json.Unmarshal([]byte(lines[0]), &mode); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(lines[1]), &initial); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(mode, map[string]string{"type": "set_interrupt_mode", "mode": "wait"}) ||
				!reflect.DeepEqual(initial, map[string]string{"type": "prompt", "message": prompt}) {
				t.Fatalf("initial RPC commands = %v, %v", mode, initial)
			}
			if !reflect.DeepEqual(messages, printMessages) || !reflect.DeepEqual(result, printResult) {
				t.Fatalf("RPC differs from print mode:\nmessages %+v vs %+v\nresult %+v vs %+v",
					messages, printMessages, result, printResult)
			}
		})
	}
}

func TestOmpExecuteSteersUntilSessionSettled(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "steers")
	eofPath := filepath.Join(dir, "eof")
	fakePath := filepath.Join(dir, "omp")
	script := `#!/bin/sh
IFS= read -r mode || exit 11
IFS= read -r prompt || exit 12
printf '%s\n' '{"type":"agent_start"}'
IFS= read -r first || exit 13
IFS= read -r second || exit 14
printf '%s\n' "$first" "$second" > "$INPUT_PATH"
printf '%s\n' '{"type":"agent_end","messages":[],"yielded":true}' '{"type":"prompt_result","status":"completed","sessionSettled":false}'
printf '%s\n' '{"type":"agent_start"}' '{"type":"turn_start"}'
printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"handled the notices"}}'
printf '%s\n' '{"type":"agent_end","messages":[]}' '{"type":"prompt_result","status":"completed","sessionSettled":true}'
printf '%s\n' '{"type":"session_settled"}'
if IFS= read -r extra; then exit 15; fi
printf 'eof' > "$EOF_PATH"
`
	writeTestExecutable(t, fakePath, []byte(script))
	backend, err := ResolveBackend("omp", Config{
		ExecutablePath: fakePath, Logger: slog.Default(),
		Env: map[string]string{"INPUT_PATH": inputPath, "EOF_PATH": eofPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := backend.Execute(t.Context(), "original task", ExecOptions{
		ResumeSessionID: filepath.Join(dir, "session.jsonl"),
		Timeout:         10 * time.Second, EnableTaskSupplement: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.Supplement == nil || session.SupplementReady == nil {
		t.Fatal("negotiated omp session does not expose input")
	}
	select {
	case message := <-session.Messages:
		if message.Type != MessageStatus || message.Status != "running" || !session.SupplementReady() {
			t.Fatalf("agent_start did not enable supplements: %+v", message)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("omp did not start")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := session.Supplement(cancelled, "must not be sent"); err != context.Canceled {
		t.Fatalf("cancelled input error = %v", err)
	}
	instructions := []string{
		"Brian says \"keep it\"\non the next line 👀 " + strings.Repeat("x", 128*1024),
		"Brian changed the answer\n" + strings.Repeat("y", 128*1024),
	}
	// Frames larger than PIPE_BUF must stay intact even with concurrent callers.
	var writers sync.WaitGroup
	errors := make(chan error, len(instructions))
	for _, instruction := range instructions {
		writers.Add(1)
		go func() {
			defer writers.Done()
			errors <- session.Supplement(t.Context(), instruction)
		}()
	}
	writers.Wait()
	for range instructions {
		if err := <-errors; err != nil {
			t.Fatalf("steer: %v", err)
		}
	}
	var messages []Message
	for message := range session.Messages {
		messages = append(messages, message)
	}
	result := waitPiResult(t, session, time.Second)
	if result.Status != "completed" || result.Output != "handled the notices" {
		t.Fatalf("result = %+v", result)
	}
	if len(messages) != 2 || messages[0].Type != MessageStatus || messages[1].Type != MessageText {
		t.Fatalf("follow-up run events = %+v", messages)
	}
	if session.SupplementReady() {
		t.Fatal("settled session still accepts input")
	}
	if err := session.Supplement(t.Context(), "too late"); err == nil {
		t.Fatal("supplement succeeded after settlement")
	}
	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	if len(lines) != len(instructions) {
		t.Fatalf("got %d steer lines, want %d", len(lines), len(instructions))
	}
	received := make(map[string]bool)
	for _, line := range lines {
		var frame map[string]string
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("split/interleaved frame: %v", err)
		}
		if frame["type"] != "steer" || len(frame) != 2 {
			t.Fatalf("unexpected frame shape: %v", frame)
		}
		received[frame["message"]] = true
	}
	for _, instruction := range instructions {
		if !received[instruction] {
			t.Fatal("steer frame changed or lost its instruction")
		}
	}
	if eof, err := os.ReadFile(eofPath); err != nil || string(eof) != "eof" {
		t.Fatalf("session_settled did not close stdin: %q, %v", eof, err)
	}
}

func newOmpSettleTestBackend(t *testing.T, script string, env map[string]string, grace time.Duration) *piBackend {
	t.Helper()
	fakePath := filepath.Join(t.TempDir(), "omp")
	writeTestExecutable(t, fakePath, []byte(script))
	backend, err := ResolveBackend("omp", Config{ExecutablePath: fakePath, Logger: slog.Default(), Env: env})
	if err != nil {
		t.Fatal(err)
	}
	omp, ok := backend.(*piBackend)
	if !ok {
		t.Fatalf("ResolveBackend(omp) returned %T, want *piBackend", backend)
	}
	omp.settleGrace = grace
	return omp
}

// An omp that ends its turn and then never reports session_settled must not
// hold the run open: its input closes after the grace, omp exits on EOF, and
// the run completes with the turn's output.
func TestOmpExecuteClosesInputWhenSessionNeverSettles(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}
	dir := t.TempDir()
	eofPath := filepath.Join(dir, "eof")
	script := `#!/bin/sh
IFS= read -r mode || exit 11
IFS= read -r prompt || exit 12
printf '%s\n' '{"type":"agent_start"}' '{"type":"turn_start"}'
printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"final reply"}}'
printf '%s\n' '{"type":"agent_end","messages":[]}' '{"type":"prompt_result","status":"completed","sessionSettled":false}'
if IFS= read -r extra; then exit 13; fi
printf 'eof' > "$EOF_PATH"
`
	backend := newOmpSettleTestBackend(t, script, map[string]string{"EOF_PATH": eofPath}, 200*time.Millisecond)
	session, err := backend.Execute(t.Context(), "task", ExecOptions{
		ResumeSessionID: filepath.Join(dir, "session.jsonl"),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	result := waitPiResult(t, session, 5*time.Second)
	if result.Status != "completed" || result.Output != "final reply" {
		t.Fatalf("result = %+v", result)
	}
	if eof, err := os.ReadFile(eofPath); err != nil || string(eof) != "eof" {
		t.Fatalf("unsettled session kept its input open: %q, %v", eof, err)
	}
}

// A follow-up turn that starts after agent_end stops the settle timer, so a
// long follow-up keeps its input and still accepts steer frames.
func TestOmpExecuteFollowUpTurnKeepsInputOpenPastSettleGrace(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}
	dir := t.TempDir()
	steerPath := filepath.Join(dir, "steer")
	script := `#!/bin/sh
IFS= read -r mode || exit 11
IFS= read -r prompt || exit 12
printf '%s\n' '{"type":"agent_start"}' '{"type":"agent_end","messages":[]}'
printf '%s\n' '{"type":"agent_start"}'
IFS= read -r steer || exit 13
printf '%s\n' "$steer" > "$STEER_PATH"
printf '%s\n' '{"type":"agent_end","messages":[]}' '{"type":"session_settled"}'
if IFS= read -r extra; then exit 14; fi
`
	grace := 100 * time.Millisecond
	backend := newOmpSettleTestBackend(t, script, map[string]string{"STEER_PATH": steerPath}, grace)
	session, err := backend.Execute(t.Context(), "task", ExecOptions{
		ResumeSessionID:      filepath.Join(dir, "session.jsonl"),
		Timeout:              10 * time.Second,
		EnableTaskSupplement: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for started := 0; started < 2; {
		select {
		case message := <-session.Messages:
			if message.Type == MessageStatus && message.Status == "running" {
				started++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("omp did not start the follow-up turn")
		}
	}
	time.Sleep(5 * grace)
	if !session.SupplementReady() {
		t.Fatal("follow-up turn lost its input to the settle timer")
	}
	if err := session.Supplement(t.Context(), "keep going"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	if result := waitPiResult(t, session, 5*time.Second); result.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}
	if steer, err := os.ReadFile(steerPath); err != nil || !strings.Contains(string(steer), "keep going") {
		t.Fatalf("steer frame not delivered: %q, %v", steer, err)
	}
}
