package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Each accepted connection follows the protocol's streaming lifetime: a rejected
// subscription closes its socket, while valid subscriptions keep delivering.
type sessionHerdrSocket struct {
	mu          sync.Mutex
	listener    net.Listener
	connections map[net.Conn][]map[string]string
	statuses    map[string]string
	changed     chan struct{}
	rejected    chan string
	done        sync.WaitGroup
}

func newSessionHerdrSocket(t *testing.T, f *agentSessionFollower, statuses map[string]string) *sessionHerdrSocket {
	t.Helper()
	dir, err := os.MkdirTemp("", "herdr-")
	if err != nil {
		t.Fatal(err)
	}
	f.socketPath = filepath.Join(dir, "herdr.sock")
	listener, err := net.Listen("unix", f.socketPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	h := &sessionHerdrSocket{listener: listener, connections: make(map[net.Conn][]map[string]string), statuses: statuses, changed: make(chan struct{}, 64), rejected: make(chan string, 64)}
	h.done.Add(1)
	go func() {
		defer h.done.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			h.done.Add(1)
			go h.serve(conn)
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		h.mu.Lock()
		for conn := range h.connections {
			conn.Close()
		}
		h.mu.Unlock()
		h.done.Wait()
		os.RemoveAll(dir)
	})
	return h
}

func (h *sessionHerdrSocket) serve(conn net.Conn) {
	defer h.done.Done()
	defer conn.Close()
	var request struct {
		ID     string `json:"id"`
		Params struct {
			Subscriptions []map[string]string `json:"subscriptions"`
		} `json:"params"`
	}
	if json.NewDecoder(conn).Decode(&request) != nil {
		return
	}
	h.mu.Lock()
	for _, subscription := range request.Params.Subscriptions {
		pane := subscription["pane_id"]
		if pane != "" {
			if _, ok := h.statuses[pane]; !ok {
				json.NewEncoder(conn).Encode(map[string]any{"id": request.ID, "error": map[string]string{"code": "pane_not_found", "message": "pane " + pane + " not found"}})
				h.mu.Unlock()
				h.rejected <- pane
				return
			}
		}
	}
	h.connections[conn] = request.Params.Subscriptions
	encoder := json.NewEncoder(conn)
	encoder.Encode(map[string]any{"id": request.ID, "result": map[string]string{"type": "subscription_started"}})
	for _, subscription := range request.Params.Subscriptions {
		if subscription["type"] == "pane.agent_status_changed" {
			pane := subscription["pane_id"]
			encoder.Encode(sessionEvent("pane.agent_status_changed", pane, h.statuses[pane]))
		}
	}
	h.mu.Unlock()
	h.changed <- struct{}{}
	var ignored any
	json.NewDecoder(conn).Decode(&ignored)
	h.mu.Lock()
	delete(h.connections, conn)
	h.mu.Unlock()
}

func (h *sessionHerdrSocket) emit(event herdrSessionEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	typeName := strings.Replace(event.Event, "pane_", "pane.", 1)
	for conn, subscriptions := range h.connections {
		for _, subscription := range subscriptions {
			if subscription["type"] == typeName && (subscription["pane_id"] == "" || subscription["pane_id"] == event.Data.PaneID) {
				json.NewEncoder(conn).Encode(event)
				break
			}
		}
	}
}

func (h *sessionHerdrSocket) waitSubscription(t *testing.T, kind, pane string) {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		h.mu.Lock()
		found := false
		for _, subscriptions := range h.connections {
			for _, subscription := range subscriptions {
				if subscription["type"] == kind && subscription["pane_id"] == pane {
					found = true
				}
			}
		}
		h.mu.Unlock()
		if found {
			return
		}
		select {
		case <-h.changed:
		case <-timer.C:
			t.Fatalf("no subscription for %s %s", kind, pane)
		}
	}
}

func runSessionSubscriber(t *testing.T, f *agentSessionFollower) {
	t.Helper()
	f.interval = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); f.subscribe(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("subscriber did not stop")
		}
	})
}

func waitSessionWrite(t *testing.T, client *sessionClientFixture, kind string) {
	t.Helper()
	select {
	case got := <-client.writes:
		if got != kind {
			t.Fatalf("write = %s, want %s", got, kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no %s delivered", kind)
	}
}

func TestAgentSessionFollower_RejectedPaneDoesNotPoisonReconnect(t *testing.T) {
	f, client, _, _ := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	f.sessions["stale"] = &followedAgentSession{id: "stale", pane: "missing", workspaceID: "workspace-1"}
	h := newSessionHerdrSocket(t, f, map[string]string{"w12:p0": "working"})
	runSessionSubscriber(t, f)
	waitSessionWrite(t, client, "update")
	select {
	case pane := <-h.rejected:
		if pane != "missing" {
			t.Fatal(pane)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stale pane was not rejected")
	}
	h.mu.Lock()
	h.statuses["w12:p0"] = "blocked"
	for conn := range h.connections {
		conn.Close()
	}
	h.mu.Unlock()
	waitSessionWrite(t, client, "update")
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.updates) != 2 || client.updates[0].Status != "working" || client.updates[1].Status != "blocked" || len(client.ended) != 0 {
		t.Fatalf("reconnect lost status or ended process: updates=%+v ends=%v", client.updates, client.ended)
	}
}

func TestAgentSessionFollower_FailedEndDoesNotBlockLivePane(t *testing.T) {
	f, client, _, _ := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	f.sessions["closed"] = &followedAgentSession{id: "closed", pane: "w13:p0", persistedPane: "w13:p0", workspaceID: "workspace-1", status: "unknown", repository: "appboypov/multica", branch: "work-1087"}
	client.endErr = errors.New("server unavailable")
	h := newSessionHerdrSocket(t, f, map[string]string{"w12:p0": "working", "w13:p0": "unknown"})
	runSessionSubscriber(t, f)
	waitSessionWrite(t, client, "update")
	h.waitSubscription(t, "pane.closed", "")
	h.mu.Lock()
	delete(h.statuses, "w13:p0")
	h.statuses["w12:p0"] = "blocked"
	h.mu.Unlock()
	h.emit(sessionEvent("pane_closed", "w13:p0", ""))
	waitSessionWrite(t, client, "end")
	waitSessionWrite(t, client, "update")
	f.mu.Lock()
	if session := f.sessions["closed"]; session == nil || !session.ending {
		t.Error("failed DELETE lost its retry")
	}
	f.mu.Unlock()
	client.mu.Lock()
	client.endErr = nil
	client.mu.Unlock()
	f.sweep(context.Background())
	waitSessionWrite(t, client, "end")
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.sessions["closed"]; exists || f.sessions["session-1"].status != "blocked" {
		t.Fatal("end retry or valid status lost")
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.ended) != 2 || client.ended[0] != "closed" || client.ended[1] != "closed" {
		t.Fatalf("end retries=%v", client.ended)
	}
}

func TestAgentSessionFollower_MovedPaneRetainsProcessAndObservation(t *testing.T) {
	f, client, _, _ := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	h := newSessionHerdrSocket(t, f, map[string]string{"w12:p0": "working"})
	runSessionSubscriber(t, f)
	waitSessionWrite(t, client, "update")
	h.waitSubscription(t, "pane.moved", "")
	h.mu.Lock()
	delete(h.statuses, "w12:p0")
	h.statuses["w14:p0"] = "working"
	h.mu.Unlock()
	var moved herdrSessionEvent
	if err := json.Unmarshal([]byte(`{"event":"pane_moved","data":{"previous_pane_id":"w12:p0","pane":{"pane_id":"w14:p0"}}}`), &moved); err != nil {
		t.Fatal(err)
	}
	h.emit(moved)
	waitSessionWrite(t, client, "update")
	h.waitSubscription(t, "pane.agent_status_changed", "w14:p0")
	h.emit(sessionEvent("pane_closed", "w12:p0", ""))
	h.emit(sessionEvent("pane.agent_status_changed", "w14:p0", "blocked"))
	waitSessionWrite(t, client, "update")
	f.mu.Lock()
	session := *f.sessions["session-1"]
	f.mu.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	if session.pane != "w14:p0" || session.pid != 42 || session.status != "blocked" || session.ending || len(client.ended) != 0 {
		t.Fatalf("moved process changed: %+v ends=%v", session, client.ended)
	}
	if len(client.updates) != 3 || client.updates[1].HerdrPaneID == nil || *client.updates[1].HerdrPaneID != "w14:p0" {
		t.Fatalf("move identity not persisted: %+v", client.updates)
	}
}

func TestAgentSessionFollower_MovedPanePersistenceRetriedBySweep(t *testing.T) {
	f, client, _, _ := sessionFollowerFixture(t)
	registerSessionFixture(t, f)
	client.updateErr = errors.New("server unavailable")
	var moved herdrSessionEvent
	json.Unmarshal([]byte(`{"event":"pane_moved","data":{"previous_pane_id":"w12:p0","pane":{"pane_id":"w14:p0"}}}`), &moved)
	f.handleEvent(context.Background(), moved)
	if session := f.sessions["session-1"]; session.pane != "w14:p0" || session.persistedPane != "w12:p0" || session.ending {
		t.Fatalf("failed write lost observed identity: %+v", session)
	}
	client.updateErr = nil
	f.sweep(context.Background())
	if len(client.updates) != 2 || client.updates[1].HerdrPaneID == nil || *client.updates[1].HerdrPaneID != "w14:p0" || f.sessions["session-1"].persistedPane != "w14:p0" || len(client.ended) != 0 {
		t.Fatalf("move not reconciled: updates=%+v ends=%v", client.updates, client.ended)
	}
}
