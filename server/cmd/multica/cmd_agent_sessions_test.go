package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

const sessionTestAgentID = "11111111-2222-3333-4444-555555555555"

func newAgentSessionsTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "sessions"}
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("profile", "", "")
	return cmd
}

func newAgentRegisterSessionTestCmd(t *testing.T, profile string, flags map[string]string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "register-session"}
	cmd.Flags().Int("pid", 0, "")
	cmd.Flags().String("cwd", "", "")
	cmd.Flags().String("pane", "", "")
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("profile", "", "")
	flags["profile"] = profile
	// An explicit --profile outranks the MULTICA_* variables, so the named
	// profile holds the login the test's environment names.
	if profile != "" {
		login := cli.CLIConfig{ServerURL: os.Getenv("MULTICA_SERVER_URL"), Token: os.Getenv("MULTICA_TOKEN"), WorkspaceID: os.Getenv("MULTICA_WORKSPACE_ID")}
		if err := cli.SaveCLIConfigForProfile(login, profile); err != nil {
			t.Fatalf("save profile %q: %v", profile, err)
		}
	}
	for name, value := range flags {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}
	return cmd
}

func isolateSessionCommandEnv(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv("MULTICA_DAEMON_PORT", "")
	t.Setenv(cli.TaskConfigRootEnv, "")
}

// serveSessionAPI answers the agent lookup of resolveAgent and the agent's
// sessions route, and records every request it gets.
func serveSessionAPI(t *testing.T, sessions string) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/agents":
			if got := r.URL.Query().Get("workspace_id"); got != "ws-1" {
				t.Errorf("agent lookup workspace_id = %q, want ws-1", got)
			}
			_, _ = fmt.Fprintf(w, `[{"id":%q,"name":"Mitchel"},{"id":"other","name":"Guus"}]`, sessionTestAgentID)
		case r.Method == http.MethodGet && r.URL.Path == "/api/agents/"+sessionTestAgentID+"/sessions":
			_, _ = w.Write([]byte(sessions))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setCLITestServerEnv(t, srv.URL)
	return srv, &requests
}

const twoMitchelSessions = `{"sessions":[
{"id":"s-1","kind":"terminal","pid":4242,"herdr_pane_id":"w12:p0","runtime":null,"task_id":null,"issue":null,
 "work_dir":"/work/multica","repository":"appboypov/multica","branch":"work-1057","status":"idle","started_at":"2026-10-16T08:00:00Z"},
{"id":"t-1","kind":"run","pid":null,"herdr_pane_id":null,"runtime":{"id":"rt-1","name":"Mitchel (omp)"},"task_id":"t-1",
 "issue":{"id":"i-1","identifier":"WORK-1057"},"work_dir":"/work/run","repository":null,"branch":null,"status":"working","started_at":"2026-10-16T09:00:00Z"}
]}`

func TestRunAgentSessionsTableShowsPaneAndRuntime(t *testing.T) {
	isolateSessionCommandEnv(t)
	serveSessionAPI(t, twoMitchelSessions)

	out, err := captureStdout(t, func() error { return runAgentSessions(newAgentSessionsTestCmd(), []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentSessions: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("want header and two rows, got:\n%s", out)
	}
	for _, header := range []string{"WHERE", "FOLDER", "REPOSITORY", "BRANCH", "ISSUE", "STATUS", "STARTED"} {
		if !strings.Contains(lines[0], header) {
			t.Fatalf("header %q missing: %s", header, lines[0])
		}
	}
	herdr := strings.Fields(lines[1])
	if want := []string{"w12:p0", "/work/multica", "appboypov/multica", "work-1057", "-", "idle", "2026-10-16T08:00:00Z"}; strings.Join(herdr, " ") != strings.Join(want, " ") {
		t.Fatalf("Herdr row = %q, want %q", herdr, want)
	}
	if !strings.HasPrefix(lines[2], "run on Mitchel (omp)") {
		t.Fatalf("run row must start with its runtime: %q", lines[2])
	}
	run := strings.Fields(strings.TrimPrefix(lines[2], "run on Mitchel (omp)"))
	if want := []string{"/work/run", "-", "-", "WORK-1057", "working", "2026-10-16T09:00:00Z"}; strings.Join(run, " ") != strings.Join(want, " ") {
		t.Fatalf("run row = %q, want %q", run, want)
	}
}

func TestRunAgentSessionsJSONPrintsResponseAsReceived(t *testing.T) {
	isolateSessionCommandEnv(t)
	serveSessionAPI(t, twoMitchelSessions)

	cmd := newAgentSessionsTestCmd()
	_ = cmd.Flags().Set("output", "json")
	out, err := captureStdout(t, func() error { return runAgentSessions(cmd, []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentSessions: %v", err)
	}
	var got, want any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	_ = json.Unmarshal([]byte(twoMitchelSessions), &want)
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("JSON output changed the response:\n got %s\nwant %s", gotJSON, wantJSON)
	}
}

func TestRunAgentSessionsEmpty(t *testing.T) {
	isolateSessionCommandEnv(t)
	serveSessionAPI(t, `{"sessions":[]}`)

	out, err := captureStdout(t, func() error { return runAgentSessions(newAgentSessionsTestCmd(), []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentSessions: %v", err)
	}
	if strings.TrimSpace(out) != "No active sessions." {
		t.Fatalf("table output = %q, want No active sessions.", out)
	}

	cmd := newAgentSessionsTestCmd()
	_ = cmd.Flags().Set("output", "json")
	out, err = captureStdout(t, func() error { return runAgentSessions(cmd, []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentSessions json: %v", err)
	}
	var listing map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &listing); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	if string(listing["sessions"]) != "[]" {
		t.Fatalf(`JSON sessions = %s, want []`, listing["sessions"])
	}
}

// listenDaemonProfile binds the health port of a named profile, as its daemon
// would, choosing a profile whose port nothing on this machine holds, and
// returns that profile with the listener.
func listenDaemonProfile(t *testing.T) (string, net.Listener) {
	t.Helper()
	for i := range 2000 {
		profile := fmt.Sprintf("agent-sessions-test-%d", i)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", healthPortForProfile(profile)))
		if err == nil {
			return profile, ln
		}
	}
	t.Fatal("no named-profile health port is free on this machine")
	return "", nil
}

// serveDaemonSessions starts a fake daemon on a free named profile's health
// port whose /sessions answers with status and body, and returns that profile
// with the posted bodies.
func serveDaemonSessions(t *testing.T, status int, body string) (string, *[]map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var posted []map[string]any
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sessions" {
			t.Errorf("daemon request = %s %s, want POST /sessions", r.Method, r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode daemon request: %v", err)
		}
		mu.Lock()
		posted = append(posted, request)
		mu.Unlock()
		if status == http.StatusCreated {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		http.Error(w, body, status)
	}))
	_ = srv.Listener.Close()
	profile, ln := listenDaemonProfile(t)
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return profile, &posted
}

func TestRunAgentRegisterSessionPostsToDaemonPort(t *testing.T) {
	isolateSessionCommandEnv(t)
	serveSessionAPI(t, `{"sessions":[]}`)
	profile, posted := serveDaemonSessions(t, http.StatusCreated, `{"id":"session-1"}`)
	folder := t.TempDir()

	cmd := newAgentRegisterSessionTestCmd(t, profile, map[string]string{"pid": "4242", "cwd": folder, "pane": "w12:p0"})
	out, err := captureStdout(t, func() error { return runAgentRegisterSession(cmd, []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentRegisterSession: %v", err)
	}
	if strings.TrimSpace(out) != "session-1" {
		t.Fatalf("stdout = %q, want the session id", out)
	}
	if len(*posted) != 1 {
		t.Fatalf("daemon got %d registrations, want 1", len(*posted))
	}
	want := map[string]any{"workspace_id": "ws-1", "agent_id": sessionTestAgentID, "pid": float64(4242), "cwd": folder, "herdr_pane_id": "w12:p0"}
	got := (*posted)[0]
	if len(got) != len(want) {
		t.Fatalf("registration = %v, want %v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("registration[%s] = %v, want %v (full: %v)", key, got[key], value, got)
		}
	}

	cmd = newAgentRegisterSessionTestCmd(t, profile, map[string]string{"pid": "4242", "cwd": folder, "output": "json"})
	out, err = captureStdout(t, func() error { return runAgentRegisterSession(cmd, []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentRegisterSession json: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil || result["id"] != "session-1" || len(result) != 1 {
		t.Fatalf("JSON output = %q, want {\"id\":\"session-1\"}", out)
	}
}

// A persona alias runs both commands without --profile. They reach the named
// profile whose daemon runs for its server and workspace, not the default
// profile, which points at another workspace and has no daemon.
func TestAgentSessionCommandsFollowTheRunningDaemonsProfile(t *testing.T) {
	isolateSessionCommandEnv(t)
	api, requests := serveSessionAPI(t, `{"sessions":[]}`)
	t.Setenv("MULTICA_SERVER_URL", "")
	t.Setenv("MULTICA_WORKSPACE_ID", "")
	t.Setenv("MULTICA_TOKEN", "")
	if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{ServerURL: api.URL, WorkspaceID: "unrelated-default-workspace", Token: "default-token"}, ""); err != nil {
		t.Fatalf("save default profile: %v", err)
	}

	var mu sync.Mutex
	var posted []map[string]any
	daemon := httptest.NewUnstartedServer(nil)
	_ = daemon.Listener.Close()
	profile, ln := listenDaemonProfile(t)
	daemon.Listener = ln
	daemon.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "running", "profile": profile, "server_url": api.URL,
				"workspaces": []map[string]any{{"id": "ws-1"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/sessions":
			var request map[string]any
			_ = json.NewDecoder(r.Body).Decode(&request)
			mu.Lock()
			posted = append(posted, request)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"session-1"}`))
		default:
			t.Errorf("unexpected daemon request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	daemon.Start()
	t.Cleanup(daemon.Close)
	if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{ServerURL: api.URL, WorkspaceID: "ws-1", Token: "named-token"}, profile); err != nil {
		t.Fatalf("save named profile: %v", err)
	}

	cmd := newAgentRegisterSessionTestCmd(t, "", map[string]string{"pid": "4242", "cwd": t.TempDir()})
	out, err := captureStdout(t, func() error { return runAgentRegisterSession(cmd, []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentRegisterSession: %v", err)
	}
	if strings.TrimSpace(out) != "session-1" {
		t.Fatalf("stdout = %q, want the session id", out)
	}
	mu.Lock()
	registrations := posted
	mu.Unlock()
	if len(registrations) != 1 || registrations[0]["workspace_id"] != "ws-1" || registrations[0]["agent_id"] != sessionTestAgentID {
		t.Fatalf("daemon registrations = %v, want one for Mitchel in ws-1", registrations)
	}

	out, err = captureStdout(t, func() error { return runAgentSessions(newAgentSessionsTestCmd(), []string{"mitchel"}) })
	if err != nil {
		t.Fatalf("runAgentSessions: %v", err)
	}
	if strings.TrimSpace(out) != "No active sessions." {
		t.Fatalf("sessions output = %q, want No active sessions.", out)
	}
	if want := "GET /api/agents/" + sessionTestAgentID + "/sessions"; (*requests)[len(*requests)-1] != want {
		t.Fatalf("API requests = %v, want the sessions read last", *requests)
	}
}

func TestRunAgentRegisterSessionWithoutDaemon(t *testing.T) {
	isolateSessionCommandEnv(t)
	_, requests := serveSessionAPI(t, `{"sessions":[]}`)
	// Bind and release a free port, so nothing listens on it during the run.
	profile, ln := listenDaemonProfile(t)
	_ = ln.Close()

	cmd := newAgentRegisterSessionTestCmd(t, profile, map[string]string{"pid": "4242", "cwd": t.TempDir()})
	out, err := captureStdout(t, func() error { return runAgentRegisterSession(cmd, []string{"mitchel"}) })
	if err == nil {
		t.Fatalf("register-session succeeded without a daemon: %q", out)
	}
	if err.Error() != errDaemonNotRunning.Error() {
		t.Fatalf("error = %q, want %q", err, errDaemonNotRunning)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("stdout = %q, want nothing", out)
	}
	for _, request := range *requests {
		if !strings.HasPrefix(request, http.MethodGet+" ") {
			t.Fatalf("a failed registration wrote to the server: %v", *requests)
		}
	}
}

func TestRunAgentRegisterSessionReturnsDaemonMessageOn400(t *testing.T) {
	isolateSessionCommandEnv(t)
	serveSessionAPI(t, `{"sessions":[]}`)
	profile, _ := serveDaemonSessions(t, http.StatusBadRequest, "session process is not running")

	cmd := newAgentRegisterSessionTestCmd(t, profile, map[string]string{"pid": "4242", "cwd": t.TempDir()})
	_, err := captureStdout(t, func() error { return runAgentRegisterSession(cmd, []string{"mitchel"}) })
	if err == nil || err.Error() != "session process is not running" {
		t.Fatalf("error = %v, want the daemon's message", err)
	}
}

func TestRunAgentRegisterSessionRefusesDaemonTask(t *testing.T) {
	isolateSessionCommandEnv(t)
	_, requests := serveSessionAPI(t, `{"sessions":[]}`)
	t.Setenv("MULTICA_TASK_ID", "task-1")

	cmd := newAgentRegisterSessionTestCmd(t, "agent-sessions-task-test", map[string]string{"pid": "4242", "cwd": t.TempDir()})
	err := runAgentRegisterSession(cmd, []string{"mitchel"})
	if err == nil || !strings.Contains(err.Error(), "agent register-session is not available inside a daemon-managed task") {
		t.Fatalf("error = %v, want the daemon-task refusal", err)
	}
	if len(*requests) != 0 {
		t.Fatalf("a refused registration reached the server: %v", *requests)
	}
}
