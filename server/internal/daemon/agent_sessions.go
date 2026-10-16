package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const agentSessionInterval = 30 * time.Second

var errSessionProcessGone = errors.New("session process is not running")

type sessionProcessReader interface {
	StartedAt(context.Context, int) (time.Time, error)
}

type sessionGitReader interface {
	Read(context.Context, string) (repository, branch string, err error)
}

type sessionClient interface {
	RegisterAgentSession(context.Context, AgentSessionRegistration) (string, error)
	UpdateAgentSession(context.Context, string, AgentSessionUpdate) error
	EndAgentSession(context.Context, string, string, string) error
	ListAgentSessions(context.Context, string, string) ([]db.AgentSession, error)
}

type localSessionRegistration struct {
	WorkspaceID string `json:"workspace_id"`
	AgentID     string `json:"agent_id"`
	PID         int    `json:"pid"`
	CWD         string `json:"cwd"`
	HerdrPaneID string `json:"herdr_pane_id"`
}

type followedAgentSession struct {
	id            string
	workspaceID   string
	pane          string
	persistedPane string
	pid           int
	startedAt     time.Time
	folder        string
	repository    string
	branch        string
	status        string
	ending        bool
}

type agentSessionFollower struct {
	mu         sync.Mutex
	sessions   map[string]*followedAgentSession
	loaded     map[string]bool
	client     sessionClient
	processes  sessionProcessReader
	git        sessionGitReader
	workspaces func() []string
	daemonID   string
	logger     *slog.Logger
	socketPath string
	changed    chan struct{}
	interval   time.Duration
}

func newAgentSessionFollower(client sessionClient, daemonID string, workspaces func() []string, logger *slog.Logger) *agentSessionFollower {
	socketPath := os.Getenv("HERDR_SOCKET_PATH")
	if socketPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			socketPath = filepath.Join(home, ".config", "herdr", "herdr.sock")
		}
	}
	return &agentSessionFollower{
		sessions: make(map[string]*followedAgentSession), loaded: make(map[string]bool),
		client: client, daemonID: daemonID, workspaces: workspaces, logger: logger,
		processes: localSessionProcesses{}, git: localSessionGit{}, socketPath: socketPath,
		changed: make(chan struct{}, 1), interval: agentSessionInterval,
	}
}

func (f *agentSessionFollower) notifySubscription() {
	select {
	case f.changed <- struct{}{}:
	default:
	}
}

func nullableSessionString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (f *agentSessionFollower) register(ctx context.Context, request localSessionRegistration) (string, error) {
	startedAt, err := f.processes.StartedAt(ctx, request.PID)
	if err != nil {
		return "", err
	}
	repository, branch, err := f.git.Read(ctx, request.CWD)
	if err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	id, err := f.client.RegisterAgentSession(ctx, AgentSessionRegistration{
		WorkspaceID: request.WorkspaceID, DaemonID: f.daemonID, AgentID: request.AgentID,
		PID: request.PID, ProcessStartedAt: startedAt, WorkDir: request.CWD,
		HerdrPaneID: nullableSessionString(request.HerdrPaneID), Repository: nullableSessionString(repository),
		Branch: nullableSessionString(branch), Status: "unknown",
	})
	if err != nil {
		return "", err
	}
	// The server upsert replaces the previous registration of this process.
	for existingID, session := range f.sessions {
		if session.workspaceID == request.WorkspaceID && session.pid == request.PID {
			delete(f.sessions, existingID)
		}
	}
	f.sessions[id] = &followedAgentSession{
		id: id, workspaceID: request.WorkspaceID, pane: request.HerdrPaneID, persistedPane: request.HerdrPaneID,
		pid: request.PID, startedAt: startedAt, folder: request.CWD,
		repository: repository, branch: branch, status: "unknown",
	}
	f.notifySubscription()
	return id, nil
}

// load also picks up workspaces first served after startup. Failed loads are
// retried by the next sweep; already followed registrations are never overwritten.
func (f *agentSessionFollower) load(ctx context.Context) {
	for _, workspaceID := range f.workspaces() {
		f.mu.Lock()
		if f.loaded[workspaceID] {
			f.mu.Unlock()
			continue
		}
		rows, err := f.client.ListAgentSessions(ctx, workspaceID, f.daemonID)
		if err != nil {
			f.logger.Warn("load agent sessions", "workspace_id", workspaceID, "error", err)
			f.mu.Unlock()
			continue
		}
		for _, row := range rows {
			id := util.UUIDToString(row.ID)
			if _, exists := f.sessions[id]; exists {
				continue
			}
			f.sessions[id] = &followedAgentSession{
				id: id, workspaceID: workspaceID, pane: row.HerdrPaneID.String, persistedPane: row.HerdrPaneID.String,
				pid: int(row.Pid), startedAt: row.ProcessStartedAt.Time, folder: row.WorkDir,
				repository: row.Repository.String, branch: row.Branch.String, status: row.Status,
			}
		}
		f.loaded[workspaceID] = true
		f.mu.Unlock()
		f.notifySubscription()
	}
}

func (f *agentSessionFollower) end(ctx context.Context, session *followedAgentSession) {
	// Retain failed deletes so the next sweep retries even if the pane's event
	// will never be emitted again and its shell process is still alive.
	if !session.ending {
		session.ending = true
		f.notifySubscription()
	}
	if err := f.client.EndAgentSession(ctx, session.workspaceID, f.daemonID, session.id); err != nil {
		f.logger.Warn("end agent session", "session_id", session.id, "error", err)
		return
	}
	delete(f.sessions, session.id)
	f.notifySubscription()
}

func (f *agentSessionFollower) refresh(ctx context.Context, session *followedAgentSession, status string) {
	repository, branch, err := f.git.Read(ctx, session.folder)
	if err != nil {
		f.logger.Warn("read agent session repository", "session_id", session.id, "error", err)
		return
	}
	if session.status == status && session.repository == repository && session.branch == branch && session.pane == session.persistedPane {
		return
	}
	if err := f.client.UpdateAgentSession(ctx, session.id, AgentSessionUpdate{
		WorkspaceID: session.workspaceID, DaemonID: f.daemonID, Status: status,
		Repository: nullableSessionString(repository), Branch: nullableSessionString(branch),
		HerdrPaneID: nullableSessionString(session.pane),
	}); err != nil {
		f.logger.Warn("update agent session", "session_id", session.id, "error", err)
		return
	}
	session.status, session.repository, session.branch = status, repository, branch
	session.persistedPane = session.pane
}

func (f *agentSessionFollower) sweep(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, session := range f.sessions {
		if ctx.Err() != nil {
			return
		}
		if session.ending {
			f.end(ctx, session)
			continue
		}
		startedAt, err := f.processes.StartedAt(ctx, session.pid)
		if errors.Is(err, errSessionProcessGone) || (err == nil && !startedAt.Equal(session.startedAt)) {
			f.end(ctx, session)
			continue
		}
		if err != nil {
			f.logger.Warn("read agent session process", "session_id", session.id, "error", err)
			continue
		}
		f.refresh(ctx, session, session.status)
	}
}

// Herdr protocol 22 sends subscription events as {event,data}, without the
// request response's {id,result} wrapper. Lifecycle events use underscore names;
// pane-filtered status subscriptions use the dotted event name.
type herdrSessionEvent struct {
	Event string `json:"event"`
	Data  struct {
		PaneID         string `json:"pane_id"`
		PreviousPaneID string `json:"previous_pane_id"`
		AgentStatus    string `json:"agent_status"`
		Pane           struct {
			PaneID string `json:"pane_id"`
		} `json:"pane"`
	} `json:"data"`
}

func (f *agentSessionFollower) handleEvent(ctx context.Context, event herdrSessionEvent) {
	if event.Event == "pane_moved" {
		if event.Data.PreviousPaneID == "" || event.Data.Pane.PaneID == "" {
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, session := range f.sessions {
			if !session.ending && session.pane == event.Data.PreviousPaneID {
				session.pane = event.Data.Pane.PaneID
				f.notifySubscription()
				f.refresh(ctx, session, session.status)
			}
		}
		return
	}
	if event.Data.PaneID == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, session := range f.sessions {
		if session.pane != event.Data.PaneID {
			continue
		}
		switch event.Event {
		case "pane_closed", "pane_exited":
			f.end(ctx, session)
		case "pane.agent_status_changed":
			if !session.ending {
				f.refresh(ctx, session, event.Data.AgentStatus)
			}
		}
	}
}

func (f *agentSessionFollower) run(ctx context.Context) {
	f.load(ctx)
	f.sweep(ctx)
	subscriberDone := make(chan struct{})
	go func() {
		defer close(subscriberDone)
		f.subscribe(ctx)
	}()
	defer func() { <-subscriberDone }()
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.load(ctx)
			f.sweep(ctx)
		}
	}
}

func (f *agentSessionFollower) subscribe(ctx context.Context) {
	logged := false
	for ctx.Err() == nil {
		// Drain changes already represented in the subscription we are building.
		select {
		case <-f.changed:
		default:
		}
		err := f.readHerdr(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errSessionSubscriptionChanged) {
			continue
		}
		if !logged {
			f.logger.Warn("Herdr session subscription unavailable; process sweep continues", "error", err)
			logged = true
		}
		timer := time.NewTimer(f.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-f.changed:
			timer.Stop()
		case <-timer.C:
		}
	}
}

var errSessionSubscriptionChanged = errors.New("agent session subscriptions changed")

func (f *agentSessionFollower) readHerdr(ctx context.Context) error {
	connectionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	changed := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-connectionCtx.Done():
		case <-f.changed:
			close(changed)
			cancel()
		}
	}()
	var observers sync.WaitGroup
	defer func() { cancel(); observers.Wait(); <-watcherDone }()

	// Herdr rejects an entire subscription and closes its socket when any
	// filtered pane is missing. Give each pane its own retry lifetime.
	f.mu.Lock()
	panes := make(map[string]bool)
	for _, session := range f.sessions {
		if session.pane != "" && !session.ending {
			panes[session.pane] = true
		}
	}
	f.mu.Unlock()
	for pane := range panes {
		observers.Add(1)
		go func() {
			defer observers.Done()
			f.followHerdrPane(connectionCtx, pane)
		}()
	}
	err := f.readHerdrSubscription(connectionCtx, []map[string]string{
		{"type": "pane.closed"}, {"type": "pane.exited"}, {"type": "pane.moved"},
	})
	select {
	case <-changed:
		return errSessionSubscriptionChanged
	default:
		return err
	}
}

func (f *agentSessionFollower) followHerdrPane(ctx context.Context, pane string) {
	logged := false
	for ctx.Err() == nil {
		err := f.readHerdrSubscription(ctx, []map[string]string{{"type": "pane.agent_status_changed", "pane_id": pane}})
		if ctx.Err() != nil {
			return
		}
		if !logged {
			f.logger.Warn("Herdr pane observation unavailable; process sweep continues", "pane_id", pane, "error", err)
			logged = true
		}
		timer := time.NewTimer(f.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (f *agentSessionFollower) readHerdrSubscription(ctx context.Context, subscriptions []map[string]string) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", f.socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	request := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			Subscriptions []map[string]string `json:"subscriptions"`
		} `json:"params"`
	}{ID: "multica-agent-sessions", Method: "events.subscribe"}
	request.Params.Subscriptions = subscriptions
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return err
	}
	decoder := json.NewDecoder(conn)
	for {
		var message struct {
			herdrSessionEvent
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := decoder.Decode(&message); err != nil {
			return err
		}
		if message.Error != nil {
			return fmt.Errorf("Herdr %s: %s", message.Error.Code, message.Error.Message)
		}
		f.handleEvent(ctx, message.herdrSessionEvent)
	}
}

type localSessionProcesses struct{}

func (localSessionProcesses) StartedAt(ctx context.Context, pid int) (time.Time, error) {
	if pid <= 0 {
		return time.Time{}, errSessionProcessGone
	}
	processCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(processCtx, "ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && len(strings.TrimSpace(string(out))) == 0 {
			return time.Time{}, errSessionProcessGone
		}
		return time.Time{}, fmt.Errorf("read process %d start time: %w", pid, err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return time.Time{}, errSessionProcessGone
	}
	startedAt, err := time.Parse("Mon Jan _2 15:04:05 2006", strings.TrimSpace(string(out)))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse process %d start time: %w", pid, err)
	}
	return startedAt, nil
}

type localSessionGit struct{}

func (localSessionGit) Read(ctx context.Context, folder string) (string, string, error) {
	gitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := checkoutGit(gitCtx, folder, "rev-parse", "--git-dir"); err != nil {
		if gitCtx.Err() != nil {
			return "", "", gitCtx.Err()
		}
		return "", "", nil
	}
	origin, err := checkoutGit(gitCtx, folder, "remote", "get-url", "origin")
	if err != nil && gitCtx.Err() != nil {
		return "", "", gitCtx.Err()
	}
	branch, err := checkoutGit(gitCtx, folder, "branch", "--show-current")
	if err != nil {
		return "", "", err
	}
	return execenv.GitHubFullNameFromURL(origin), branch, nil
}
