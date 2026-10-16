package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type sessionProcessFixture struct {
	mu     sync.Mutex
	starts map[int]time.Time
	err    error
}

func (p *sessionProcessFixture) StartedAt(_ context.Context, pid int) (time.Time, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return time.Time{}, p.err
	}
	start, ok := p.starts[pid]
	if !ok {
		return time.Time{}, errSessionProcessGone
	}
	return start, nil
}

func (p *sessionProcessFixture) set(pid int, start time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if start.IsZero() {
		delete(p.starts, pid)
	} else {
		p.starts[pid] = start
	}
}

type sessionGitFixture struct {
	mu         sync.Mutex
	repository string
	branch     string
}

func (g *sessionGitFixture) Read(context.Context, string) (string, string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.repository, g.branch, nil
}

func (g *sessionGitFixture) set(repository, branch string) {
	g.mu.Lock()
	g.repository, g.branch = repository, branch
	g.mu.Unlock()
}

type sessionClientFixture struct {
	mu            sync.Mutex
	registrations []AgentSessionRegistration
	updates       []AgentSessionUpdate
	ended         []string
	listed        []string
	rows          map[string][]db.AgentSession
	endErr        error
	updateErr     error
	writes        chan string
}

func (c *sessionClientFixture) RegisterAgentSession(_ context.Context, registration AgentSessionRegistration) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registrations = append(c.registrations, registration)
	return "session-1", nil
}

func (c *sessionClientFixture) UpdateAgentSession(_ context.Context, _ string, update AgentSessionUpdate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, update)
	if c.writes != nil {
		c.writes <- "update"
	}
	return c.updateErr
}

func (c *sessionClientFixture) EndAgentSession(_ context.Context, _, _, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ended = append(c.ended, id)
	if c.writes != nil {
		c.writes <- "end"
	}
	return c.endErr
}

func (c *sessionClientFixture) ListAgentSessions(_ context.Context, workspaceID, _ string) ([]db.AgentSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listed = append(c.listed, workspaceID)
	return c.rows[workspaceID], nil
}

func sessionFollowerFixture(t *testing.T) (*agentSessionFollower, *sessionClientFixture, *sessionProcessFixture, *sessionGitFixture) {
	t.Helper()
	client := &sessionClientFixture{rows: make(map[string][]db.AgentSession), writes: make(chan string, 64)}
	processes := &sessionProcessFixture{starts: map[int]time.Time{42: time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)}}
	git := &sessionGitFixture{repository: "appboypov/multica", branch: "work-1087"}
	f := newAgentSessionFollower(client, "daemon-1", func() []string { return []string{"workspace-1"} }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.processes, f.git = processes, git
	return f, client, processes, git
}

func registerSessionFixture(t *testing.T, f *agentSessionFollower) {
	t.Helper()
	if _, err := f.register(context.Background(), localSessionRegistration{WorkspaceID: "workspace-1", AgentID: "agent-1", PID: 42, CWD: "/worktree", HerdrPaneID: "w12:p0"}); err != nil {
		t.Fatal(err)
	}
}

func sessionEvent(kind, pane, status string) herdrSessionEvent {
	var event herdrSessionEvent
	event.Event, event.Data.PaneID, event.Data.AgentStatus = kind, pane, status
	return event
}

func TestAgentSessionFollower_StatusAndSweep(t *testing.T) {
	f, client, _, git := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	registration := client.registrations[0]
	if registration.ProcessStartedAt.IsZero() || registration.WorkDir != "/worktree" || registration.Repository == nil || *registration.Repository != "appboypov/multica" || registration.Status != "unknown" {
		t.Fatalf("incomplete registration: %+v", registration)
	}
	ctx := context.Background()
	f.handleEvent(ctx, sessionEvent("pane.agent_status_changed", "w12:p0", "working"))
	f.handleEvent(ctx, sessionEvent("pane.agent_status_changed", "w12:p0", "working"))
	if len(client.updates) != 1 || client.updates[0].Status != "working" {
		t.Fatalf("status writes = %+v", client.updates)
	}
	git.set("appboypov/madspec", "work-1060")
	f.sweep(ctx)
	if len(client.updates) != 2 || *client.updates[1].Branch != "work-1060" || *client.updates[1].Repository != "appboypov/madspec" {
		t.Fatalf("branch/repository not refreshed: %+v", client.updates)
	}
	git.set("", "")
	f.sweep(ctx)
	if len(client.updates) != 3 || client.updates[2].Branch != nil || client.updates[2].Repository != nil {
		t.Fatalf("cleared repository not reported: %+v", client.updates)
	}
}

func TestAgentSessionFollower_PaneEndAndIsolation(t *testing.T) {
	for _, kind := range []string{"pane_closed", "pane_exited"} {
		t.Run(kind, func(t *testing.T) {
			f, client, _, _ := sessionFollowerFixture(t)
			registerSessionFixture(t, f)
			t.Setenv("HERDR_PANE_ID", "w75:p0")
			for _, pane := range []string{"w99:p0", os.Getenv("HERDR_PANE_ID")} {
				f.handleEvent(context.Background(), sessionEvent("pane.agent_status_changed", pane, "idle"))
				f.handleEvent(context.Background(), sessionEvent(kind, pane, ""))
			}
			if len(client.updates) != 0 || len(client.ended) != 0 {
				t.Fatal("unknown/daemon pane affected followed sessions")
			}
			f.handleEvent(context.Background(), sessionEvent(kind, "w12:p0", ""))
			f.handleEvent(context.Background(), sessionEvent(kind, "w12:p0", ""))
			if len(client.ended) != 1 || len(f.sessions) != 0 {
				t.Fatalf("end calls=%v sessions=%v", client.ended, f.sessions)
			}
		})
	}
}

func TestAgentSessionFollower_ProcessGoneOrReused(t *testing.T) {
	for _, reused := range []bool{false, true} {
		t.Run(map[bool]string{false: "gone", true: "reused"}[reused], func(t *testing.T) {
			f, client, processes, _ := sessionFollowerFixture(t)
			registerSessionFixture(t, f)
			start := time.Time{}
			if reused {
				start = processes.starts[42].Add(time.Second)
			}
			processes.set(42, start)
			f.sweep(context.Background())
			if len(client.ended) != 1 || len(f.sessions) != 0 {
				t.Fatalf("dead identity retained: ends=%v", client.ended)
			}
		})
	}
}

func TestAgentSessionFollower_ProcessReadFailureDoesNotEnd(t *testing.T) {
	f, client, processes, _ := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	processes.err = errors.New("ps unavailable")
	f.sweep(context.Background())
	if len(client.ended) != 0 || len(f.sessions) != 1 {
		t.Fatal("an unreadable process was treated as dead")
	}
}

func TestAgentSessionFollower_FailedPaneEndRetriedBySweep(t *testing.T) {
	f, client, _, _ := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	client.endErr = errors.New("server unavailable")
	f.handleEvent(context.Background(), sessionEvent("pane_closed", "w12:p0", ""))
	client.endErr = nil
	f.sweep(context.Background())
	if len(client.ended) != 2 || len(f.sessions) != 0 {
		t.Fatal("failed pane end was not reconciled")
	}
}

func TestAgentSessionFollower_LoadsEachWorkspaceAndSweepsImmediately(t *testing.T) {
	f, client, _, _ := sessionFollowerFixture(t)
	id := uuid.New()
	client.rows["workspace-1"] = []db.AgentSession{{ID: pgtype.UUID{Bytes: id, Valid: true}, Pid: 900, WorkDir: "/old", ProcessStartedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}}
	f.workspaces = func() []string { return []string{"workspace-1", "workspace-2"} }
	f.socketPath = filepath.Join(t.TempDir(), "missing.sock")
	// A one-hour interval proves the first sweep is not waiting for its ticker.
	f.interval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { f.run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("follower did not stop")
		}
	})
	select {
	case write := <-client.writes:
		if write != "end" {
			t.Fatalf("write=%s", write)
		}
	case <-time.After(time.Second):
		t.Fatal("startup did not immediately end missing process")
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if !reflect.DeepEqual(client.listed, []string{"workspace-1", "workspace-2"}) || len(client.ended) != 1 || client.ended[0] != id.String() {
		t.Fatalf("startup list=%v end=%v", client.listed, client.ended)
	}
}

func TestAgentSessionFollower_HerdrReconnectWhileSweepContinues(t *testing.T) {
	f, client, processes, git := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	f.interval = 20 * time.Millisecond
	f.socketPath = filepath.Join(t.TempDir(), "herdr.sock")
	listener, err := net.Listen("unix", f.socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	disconnect := make(chan struct{})
	var disconnectOnce sync.Once
	closeConnection := func() { disconnectOnce.Do(func() { close(disconnect) }) }
	t.Cleanup(closeConnection)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var request struct {
			ID     string `json:"id"`
			Method string `json:"method"`
			Params struct {
				Subscriptions []map[string]string `json:"subscriptions"`
			} `json:"params"`
		}
		if json.NewDecoder(conn).Decode(&request) != nil {
			return
		}
		if request.Method != "events.subscribe" {
			return
		}
		encoder := json.NewEncoder(conn)
		encoder.Encode(map[string]any{"id": request.ID, "result": map[string]string{"type": "subscription_started"}})
		encoder.Encode(sessionEvent("pane.agent_status_changed", "w12:p0", "working"))
		<-disconnect
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { f.run(ctx); close(done) }()
	t.Cleanup(func() {
		closeConnection()
		cancel()
		listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("follower did not stop")
		}
		select {
		case <-serverDone:
		case <-time.After(time.Second):
			t.Error("socket server did not stop")
		}
	})
	select {
	case <-client.writes:
	case <-time.After(time.Second):
		t.Fatal("Herdr event did not update")
	}
	// Remove the listener while the existing connection is lost. The process
	// sweep must still report a branch change with no Herdr socket available.
	listener.Close()
	closeConnection()
	git.set("appboypov/multica", "offline-branch")
	select {
	case <-client.writes:
	case <-time.After(time.Second):
		t.Fatal("sweep stopped with Herdr disconnected")
	}
	replacement, err := net.Listen("unix", f.socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	// Serve the replacement socket to observe a genuine reconnect.
	reconnected := make(chan struct{})
	go func() {
		conn, err := replacement.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var request map[string]any
		if json.NewDecoder(conn).Decode(&request) == nil {
			close(reconnected)
		}
		io.Copy(io.Discard, conn)
	}()
	select {
	case <-reconnected:
	case <-time.After(time.Second):
		t.Fatal("Herdr did not reconnect")
	}
	processes.set(42, time.Time{})
	select {
	case write := <-client.writes:
		if write != "end" {
			t.Fatalf("write=%s", write)
		}
	case <-time.After(time.Second):
		t.Fatal("sweep did not end process after reconnect")
	}
}

func TestAgentSessionLocalRoute(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"register", `{"workspace_id":"workspace-1","agent_id":"agent-1","pid":42,"cwd":"/worktree","herdr_pane_id":"w12:p0"}`, http.StatusCreated},
		{"workspace", `{"workspace_id":"other","agent_id":"agent-1","pid":42,"cwd":"/worktree"}`, http.StatusBadRequest},
		{"process", `{"workspace_id":"workspace-1","agent_id":"agent-1","pid":900,"cwd":"/worktree"}`, http.StatusBadRequest},
		{"invalid", `{`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, client, _, _ := sessionFollowerFixture(t)
			d := &Daemon{sessionFollower: f, logger: f.logger, workspaces: map[string]*workspaceState{"workspace-1": {}}}
			response := httptest.NewRecorder()
			d.sessionsHandler()(response, httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(tc.body)))
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			if tc.status == http.StatusCreated {
				var body struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.ID != "session-1" {
					t.Fatalf("registration response=%s", response.Body)
				}
			} else if len(client.registrations) != 0 {
				t.Fatal("invalid request reached server")
			}
		})
	}
}

func TestAgentSessionLocalReaders(t *testing.T) {
	start, err := (localSessionProcesses{}).StartedAt(context.Background(), os.Getpid())
	if err != nil || start.IsZero() || start.After(time.Now()) {
		t.Fatalf("real process identity: %s %v", start, err)
	}
	again, err := (localSessionProcesses{}).StartedAt(context.Background(), os.Getpid())
	if err != nil || !again.Equal(start) {
		t.Fatalf("unstable process identity: %s %s %v", start, again, err)
	}
	if _, err := (localSessionProcesses{}).StartedAt(context.Background(), -1); !errors.Is(err, errSessionProcessGone) {
		t.Fatalf("invalid PID: %v", err)
	}
	folder := t.TempDir()
	git := localSessionGit{}
	if repository, branch, err := git.Read(context.Background(), folder); err != nil || repository != "" || branch != "" {
		t.Fatalf("non-repository: %q %q %v", repository, branch, err)
	}
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", folder}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, output, err)
		}
	}
	run("init", "-b", "work-1087")
	run("remote", "add", "origin", "git@github.com:appboypov/multica.git")
	if repository, branch, err := git.Read(context.Background(), folder); err != nil || repository != "appboypov/multica" || branch != "work-1087" {
		t.Fatalf("SSH repository: %q %q %v", repository, branch, err)
	}
	run("remote", "set-url", "origin", "https://github.com/appboypov/madspec.git")
	if repository, _, err := git.Read(context.Background(), folder); err != nil || repository != "appboypov/madspec" {
		t.Fatalf("HTTPS repository: %q %v", repository, err)
	}
}
