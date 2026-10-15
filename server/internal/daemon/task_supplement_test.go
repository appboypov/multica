package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func taskSupplementTestDaemon(t *testing.T, handler http.HandlerFunc) *Daemon {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Daemon{
		client:                      NewClient(server.URL),
		logger:                      slog.New(slog.NewTextHandler(io.Discard, nil)),
		taskSupplementPollInterval:  10 * time.Millisecond,
		taskSupplementReadyInterval: 5 * time.Millisecond,
	}
}

func TestTaskSupplementNoticeNamesItemAndReactions(t *testing.T) {
	for _, tc := range []struct {
		kind string
		name string
	}{
		{kind: "comment", name: "comment"},
		{kind: "comment_edit", name: "edited comment"},
		{kind: "answer", name: "answer"},
		{kind: "issue_change", name: "issue change"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			supplement := &TaskSupplement{
				ItemKind: tc.kind, ItemID: "item-" + tc.kind,
				CommentID: "original-comment", AuthorName: " Brian\n de App ",
				Content: "Use the toolbar copy button.\nThread: thread-1 (Skuddy's thread)\ntoolbar.png (attachment file-1, multica attachment download file-1)",
			}
			instruction := formatTaskSupplementInstruction("WORK-434", supplement)
			for _, want := range []string{
				"[Multica notice] " + tc.name,
				`"Brian de App"`, "WORK-434", "(item " + supplement.ItemID + ")",
				supplement.Content,
				"multica issue react " + supplement.ItemID + " 👀",
				"multica issue react " + supplement.ItemID + " ✅",
				"relevant to your work", "leave it unmarked",
			} {
				if !strings.Contains(instruction, want) {
					t.Errorf("notice lost %q: %q", want, instruction)
				}
			}
			if strings.Contains(instruction, "multica issue react original-comment") {
				t.Errorf("notice reacts to the comment instead of the received item: %q", instruction)
			}
		})
	}
}

type runInputSessionBackend struct {
	session *agent.Session
}

func (b runInputSessionBackend) Execute(context.Context, string, agent.ExecOptions) (*agent.Session, error) {
	return b.session, nil
}

func TestExecuteAndDrainDeliversEveryRunInputKind(t *testing.T) {
	items := []protocol.TaskSupplementClaimResponse{
		{ItemKind: "comment", ItemID: "comment-1", CommentID: "comment-1", AuthorName: "Brian", Content: "Keep the toggle.\nThread: thread-1 (Skuddy's thread)"},
		{ItemKind: "comment_edit", ItemID: "edit-1", CommentID: "comment-1", AuthorName: "Brian", Content: "Keep the toggle in the header.\ntoolbar.png (attachment file-1, multica attachment download file-1)"},
		{ItemKind: "answer", ItemID: "answer-1", AuthorName: "Brian", Content: `Brian answered Skuddy's question "Which button?": Use copy.`},
		{ItemKind: "issue_change", ItemID: "change-1", AuthorName: "Skuddy", Content: "Skuddy changed the priority from Medium to High"},
	}
	var claims atomic.Int32
	var acknowledgements atomic.Int32
	results := make(chan agent.Result, 1)
	d := taskSupplementTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("request method = %s, want POST", r.Method)
		}
		if r.URL.Path == "/api/daemon/tasks/task-items/supplements/claim" {
			index := int(claims.Add(1)) - 1
			if got := int(acknowledgements.Load()); got != index {
				t.Errorf("claim %d preceded acknowledgement of the previous item: %d", index, got)
			}
			if index == len(items) {
				results <- agent.Result{Status: "completed", Output: "done"}
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			if index > len(items) {
				t.Errorf("unexpected extra claim %d", index)
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			if err := json.NewEncoder(w).Encode(items[index]); err != nil {
				t.Error(err)
			}
			return
		}
		index := int(acknowledgements.Load())
		if index >= len(items) {
			t.Errorf("unexpected extra acknowledgement: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		wantPath := "/api/daemon/tasks/task-items/supplements/" + items[index].ItemID + "/ack"
		if r.URL.Path != wantPath {
			t.Errorf("ack path = %s, want %s", r.URL.Path, wantPath)
		}
		var body struct {
			Delivered bool   `json:"delivered"`
			Error     string `json:"error"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !body.Delivered || body.Error != "" {
			t.Errorf("successful injection acknowledged as %+v", body)
		}
		acknowledgements.Add(1)
	})
	messages := make(chan agent.Message)
	close(messages)
	injections := 0
	session := &agent.Session{
		Messages: messages, Result: results,
		SupplementReady: func() bool { return true },
		Supplement: func(_ context.Context, instruction string) error {
			index := injections
			injections++
			if index >= len(items) {
				t.Errorf("unexpected extra injection: %q", instruction)
				return nil
			}
			item := items[index]
			if !strings.Contains(instruction, item.Content) || !strings.Contains(instruction, "WORK-434") ||
				!strings.Contains(instruction, "multica issue react "+item.ItemID+" ✅") {
				t.Errorf("%s notice lost its snapshot, run issue key or item identity: %q", item.ItemKind, instruction)
			}
			return nil
		},
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	result, _, err := d.executeAndDrain(ctx, runInputSessionBackend{session: session}, "original objective",
		agent.ExecOptions{EnableTaskSupplement: true}, d.logger, "task-items", "WORK-434", "", new(atomic.Int32))
	if err != nil || result.Status != "completed" {
		t.Fatalf("execution = %+v, error = %v", result, err)
	}
	if injections != len(items) || int(acknowledgements.Load()) != len(items) {
		t.Fatalf("injections=%d acknowledgements=%d, want %d each", injections, acknowledgements.Load(), len(items))
	}
}

func TestClaimTaskSupplementEmptyResponse(t *testing.T) {
	d := taskSupplementTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Error(err)
		}
	})
	supplement, err := d.client.ClaimTaskSupplement(t.Context(), "task-empty")
	if err != nil || supplement != nil {
		t.Fatalf("empty claim = %+v, error = %v", supplement, err)
	}
}

func TestTaskSupplementLoopWaitsForTurnReadyBeforeClaim(t *testing.T) {
	var claims atomic.Int32
	d := taskSupplementTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		claims.Add(1)
		w.WriteHeader(http.StatusNotFound)
	})
	var ready atomic.Bool
	session := &agent.Session{
		SupplementReady: ready.Load,
		Supplement:      func(context.Context, string) error { return nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	wakeup, unsubscribe := d.taskSupplementSignals.subscribe("task-ready")
	defer unsubscribe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.runTaskSupplementLoop(ctx, session, "task-ready", "WORK-438", wakeup, d.logger)
	}()
	time.Sleep(30 * time.Millisecond)
	if got := claims.Load(); got != 0 {
		t.Fatalf("claims before turn ready = %d, want 0", got)
	}
	ready.Store(true)
	d.taskSupplementSignals.notify("task-ready")
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop did not stop after old-server 404")
	}
	if got := claims.Load(); got != 1 {
		t.Fatalf("claims after turn ready = %d, want 1", got)
	}
}

func TestTaskSupplementLoopAcknowledgesBeforeTurnEnds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		reason string
	}{
		{name: "delivered"},
		{name: "provider timeout", err: context.DeadlineExceeded, reason: protocol.TaskSupplementFailureTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var claims atomic.Int32
			ackSeen := make(chan struct{}, 1)
			d := taskSupplementTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/supplements/claim"):
					if claims.Add(1) == 1 {
						_ = json.NewEncoder(w).Encode(map[string]string{
							"item_kind": "comment", "item_id": "comment-1",
							"comment_id": "comment-1", "author_name": "Ada", "content": "Create evidence.txt",
						})
						return
					}
					w.WriteHeader(http.StatusPreconditionFailed)
				case strings.HasSuffix(r.URL.Path, "/supplements/comment-1/ack"):
					var body struct {
						Delivered bool   `json:"delivered"`
						Error     string `json:"error"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.Delivered != (tc.err == nil) || body.Error != tc.reason {
						t.Errorf("ack = %#v, want delivered=%v reason=%q", body, tc.err == nil, tc.reason)
					}
					ackSeen <- struct{}{}
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			injections := 0
			session := &agent.Session{
				SupplementReady: func() bool { return true },
				Supplement: func(_ context.Context, instruction string) error {
					injections++
					if !strings.Contains(instruction, "Preserve and complete the original objective") || !strings.Contains(instruction, "Create evidence.txt") {
						t.Errorf("injected instruction lost framing or content: %q", instruction)
					}
					return tc.err
				},
			}
			wakeup, unsubscribe := d.taskSupplementSignals.subscribe("task-ack")
			defer unsubscribe()
			d.runTaskSupplementLoop(t.Context(), session, "task-ack", "WORK-438", wakeup, d.logger)
			if injections != 1 {
				t.Fatalf("injections = %d, want 1", injections)
			}
			select {
			case <-ackSeen:
			default:
				t.Fatal("loop returned before delivery acknowledgement")
			}
		})
	}
}

type supplementGateBackend struct {
	supplementCalls atomic.Int32
	enabled         bool
}

func (b *supplementGateBackend) Execute(_ context.Context, _ string, opts agent.ExecOptions) (*agent.Session, error) {
	b.enabled = opts.EnableTaskSupplement
	messages := make(chan agent.Message)
	close(messages)
	results := make(chan agent.Result, 1)
	results <- agent.Result{Status: "completed", Output: "done"}
	return &agent.Session{
		Messages:        messages,
		Result:          results,
		SupplementReady: func() bool { return true },
		Supplement: func(context.Context, string) error {
			b.supplementCalls.Add(1)
			return nil
		},
	}, nil
}

func TestExecuteAndDrainSupplementNegotiation(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		t.Run(fmt.Sprintf("negotiated=%v", negotiated), func(t *testing.T) {
			var requests atomic.Int32
			d := taskSupplementTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusNotFound)
			})
			backend := &supplementGateBackend{}
			opts := agent.ExecOptions{EnableTaskSupplement: negotiated}
			if _, _, err := d.executeAndDrain(t.Context(), backend, "original", opts, d.logger, "task", "", "", new(atomic.Int32)); err != nil {
				t.Fatal(err)
			}
			if backend.enabled != negotiated {
				t.Fatalf("provider hooks enabled=%v, want %v", backend.enabled, negotiated)
			}
			if !negotiated && (requests.Load() != 0 || backend.supplementCalls.Load() != 0) {
				t.Fatalf("unnegotiated run made HTTP requests=%d supplement calls=%d", requests.Load(), backend.supplementCalls.Load())
			}
		})
	}
}
