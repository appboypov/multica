package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A herdr-core socket that answers events.subscribe with one terminal in its
// snapshot and then sends the given events.
func fakeHcSocket(t *testing.T, f *agentSessionFollower, snapshotState string, events ...map[string]any) {
	t.Helper()
	dir, err := os.MkdirTemp("", "hc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	f.hcSocketPath = filepath.Join(dir, "hc.sock")
	listener, err := net.Listen("unix", f.hcSocketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if json.NewDecoder(conn).Decode(&request) != nil || request.Method != "events.subscribe" {
			return
		}
		encoder := json.NewEncoder(conn)
		encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{
			"channel": nil, "seq": 1,
			"terminals": []map[string]any{{"address": "t10", "status": map[string]string{"app": "omp", "state": snapshotState}}},
		}})
		for _, event := range events {
			encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "event", "params": event})
		}
		buffer := make([]byte, 1)
		conn.Read(buffer)
	}()
}

func runFollower(t *testing.T, f *agentSessionFollower) {
	t.Helper()
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
}

func nextWrite(t *testing.T, client *sessionClientFixture, want string) {
	t.Helper()
	select {
	case got := <-client.writes:
		if got != want {
			t.Fatalf("write = %s, want %s", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("no %s write", want)
	}
}

func TestAgentSessionFollower_HerdrAppTerminal(t *testing.T) {
	f, client, _, _ := sessionFollowerFixture(t)
	f.socketPath = filepath.Join(t.TempDir(), "no-herdr.sock")
	if _, err := f.register(context.Background(), localSessionRegistration{WorkspaceID: "workspace-1", AgentID: "agent-1", PID: 42, CWD: "/worktree", HcTerminal: "t10"}); err != nil {
		t.Fatal(err)
	}
	if got := client.registrations[0].HcTerminal; got == nil || *got != "t10" {
		t.Fatalf("registered hc terminal = %v, want t10", got)
	}
	fakeHcSocket(t, f, "working",
		map[string]any{"type": "status", "terminal": "t11", "seq": 2, "data": map[string]any{"status": map[string]string{"state": "idle"}}},
		map[string]any{"type": "status", "terminal": "t10", "seq": 3, "data": map[string]any{"status": map[string]string{"state": "blocked"}}},
		map[string]any{"type": "status", "terminal": "t10", "seq": 4, "data": map[string]any{"status": map[string]string{"state": "error"}}},
		map[string]any{"type": "finished", "terminal": "t10", "seq": 5, "data": map[string]any{"exit_code": 0}},
	)
	runFollower(t, f)
	nextWrite(t, client, "update")
	nextWrite(t, client, "update")
	nextWrite(t, client, "update")
	nextWrite(t, client, "end")
	client.mu.Lock()
	defer client.mu.Unlock()
	statuses := []string{}
	for _, update := range client.updates {
		statuses = append(statuses, update.Status)
	}
	if len(statuses) != 3 || statuses[0] != "working" || statuses[1] != "blocked" || statuses[2] != "unknown" {
		t.Fatalf("statuses = %v, want snapshot working, blocked, then error as unknown", statuses)
	}
}
