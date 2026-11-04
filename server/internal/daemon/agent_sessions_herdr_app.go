package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// herdr-core (`hc`), behind the Herdr app, speaks JSON-RPC 2.0 lines. One
// events.subscribe answers with a snapshot of every terminal and then sends
// each change as an "event" notification; "missed" means the subscriber fell
// behind and needs a fresh snapshot.
type hcTerminalStatus struct {
	State string `json:"state"`
}

type hcSessionFrame struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Result *struct {
		Terminals []struct {
			Address string            `json:"address"`
			Status  *hcTerminalStatus `json:"status"`
		} `json:"terminals"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Params struct {
		Type     string `json:"type"`
		Terminal string `json:"terminal"`
		Data     struct {
			Status *hcTerminalStatus `json:"status"`
		} `json:"data"`
	} `json:"params"`
}

var errHcSnapshotMissed = errors.New("herdr-core subscription fell behind")

// hcSessionStatus maps a terminal's OSC 7501 state to a session status. A
// terminal without one, or in error, is unknown.
func hcSessionStatus(status *hcTerminalStatus) string {
	if status == nil {
		return "unknown"
	}
	switch status.State {
	case "idle", "working", "blocked", "done":
		return status.State
	default:
		return "unknown"
	}
}

func (f *agentSessionFollower) hasHcSessions() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, session := range f.sessions {
		if session.hcTerminal != "" && !session.ending {
			return true
		}
	}
	return false
}

// subscribeHc follows the Herdr app while a session runs in one of its
// terminals. Every change resubscribes, so a new registration gets its status
// from the fresh snapshot.
func (f *agentSessionFollower) subscribeHc(ctx context.Context) {
	logged := false
	for ctx.Err() == nil {
		select {
		case <-f.hcChanged:
		default:
		}
		if !f.hasHcSessions() {
			select {
			case <-ctx.Done():
				return
			case <-f.hcChanged:
				continue
			}
		}
		err := f.readHc(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errSessionSubscriptionChanged) || errors.Is(err, errHcSnapshotMissed) {
			continue
		}
		if !logged {
			f.logger.Warn("herdr-core session subscription unavailable; process sweep continues", "error", err)
			logged = true
		}
		timer := time.NewTimer(f.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-f.hcChanged:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (f *agentSessionFollower) readHc(ctx context.Context) error {
	connectionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	changed := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-connectionCtx.Done():
		case <-f.hcChanged:
			close(changed)
			cancel()
		}
	}()
	defer func() { cancel(); <-watcherDone }()
	err := f.readHcSubscription(connectionCtx)
	select {
	case <-changed:
		return errSessionSubscriptionChanged
	default:
		return err
	}
}

func (f *agentSessionFollower) readHcSubscription(ctx context.Context) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", f.hcSocketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "events.subscribe", "params": map[string]any{}}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return err
	}
	decoder := json.NewDecoder(conn)
	for {
		var frame hcSessionFrame
		if err := decoder.Decode(&frame); err != nil {
			return err
		}
		if frame.Error != nil {
			return fmt.Errorf("herdr-core %d: %s", frame.Error.Code, frame.Error.Message)
		}
		if frame.Result != nil {
			for _, terminal := range frame.Result.Terminals {
				f.handleHcEvent(ctx, "status", terminal.Address, terminal.Status)
			}
			continue
		}
		switch frame.Method {
		case "missed":
			return errHcSnapshotMissed
		case "event":
			f.handleHcEvent(ctx, frame.Params.Type, frame.Params.Terminal, frame.Params.Data.Status)
		}
	}
}

// handleHcEvent refreshes the sessions of a terminal on its status and ends
// them when the terminal finishes or is removed.
func (f *agentSessionFollower) handleHcEvent(ctx context.Context, kind, address string, status *hcTerminalStatus) {
	if address == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, session := range f.sessions {
		if session.hcTerminal != address {
			continue
		}
		switch kind {
		case "status":
			if !session.ending {
				f.refresh(ctx, session, hcSessionStatus(status))
			}
		case "finished", "removed":
			f.end(ctx, session)
		}
	}
}
