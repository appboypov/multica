//go:build !windows

package execenv

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestPrepareIsolated_PermanentFIFOBlockThenImmediateRetry exercises a real
// Codex Prepare blocked reading a shared config FIFO. Cancellation waits until
// the helper opens the FIFO. No reader may remain when an immediate retry
// mutates the same env root.
func TestPrepareIsolated_PermanentFIFOBlockThenImmediateRetry(t *testing.T) {
	sharedCodexHome := t.TempDir()
	t.Setenv("CODEX_HOME", sharedCodexHome)
	fifo := filepath.Join(sharedCodexHome, "config.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("create config FIFO: %v", err)
	}

	params := PrepareParams{
		WorkspacesRoot: t.TempDir(),
		WorkspaceID:    "ws-isolated-prepare",
		TaskID:         "11111111-2222-3333-4444-555555555555",
		AgentName:      "isolation-test",
		Provider:       "codex",
		Task: TaskContextForEnv{
			IssueID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			AgentID: "ffffffff-1111-2222-3333-444444444444",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var prepareErr error
	go func() {
		_, prepareErr = PrepareIsolated(ctx, preparationHelperTestCommand(), params, logger)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Opening a non-blocking writer proves the helper reached the FIFO.
	// Keep it open without writing so the helper cannot finish its read.
	startup := time.NewTimer(10 * time.Second)
	defer startup.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		blockedWriter, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			defer blockedWriter.Close()
			break
		}
		if !errors.Is(err, syscall.ENXIO) {
			t.Fatalf("wait for FIFO reader: %v", err)
		}
		select {
		case <-done:
			t.Fatalf("PrepareIsolated returned before blocking on FIFO: %v", prepareErr)
		case <-startup.C:
			t.Fatal("preparation helper did not open the config FIFO")
		case <-poll.C:
		}
	}

	startedAt := time.Now()
	cancel()
	<-done
	if !errors.Is(prepareErr, context.Canceled) {
		t.Fatalf("PrepareIsolated error = %v, want context canceled", prepareErr)
	}
	if elapsed := time.Since(startedAt); elapsed > 3*time.Second {
		t.Fatalf("PrepareIsolated cancellation took %s, want a bounded process termination", elapsed)
	}
	if _, err := os.Stat(PredictRootDir(RootDirParams{WorkspacesRoot: params.WorkspacesRoot, WorkspaceID: params.WorkspaceID, WorkspaceSlug: params.WorkspaceSlug, TaskID: params.TaskID, IssueIdentifier: params.IssueIdentifier})); err != nil {
		t.Fatalf("first helper did not reach environment preparation before cancellation: %v", err)
	}

	// A non-blocking FIFO writer succeeds only while a reader is still alive.
	// ENXIO therefore proves the canceled helper was reaped before ownership
	// returned to the caller; there is no old worker that can resume later.
	writer, writerErr := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if writerErr == nil {
		writer.Close()
		t.Fatal("canceled preparation helper still has the FIFO open for reading")
	}
	if !errors.Is(writerErr, syscall.ENXIO) {
		t.Fatalf("probe FIFO reader: %v, want ENXIO", writerErr)
	}

	// Retry immediately against the same root. It must be the sole writer and
	// complete successfully after the pathological source is replaced.
	if err := os.Remove(fifo); err != nil {
		t.Fatalf("remove config FIFO: %v", err)
	}
	if err := os.WriteFile(fifo, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("replace config FIFO: %v", err)
	}
	retryCtx, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer retryCancel()
	env, err := PrepareIsolated(retryCtx, preparationHelperTestCommand(), params, logger)
	if err != nil {
		t.Fatalf("immediate retry PrepareIsolated: %v", err)
	}
	if env == nil || env.RootDir != PredictRootDir(RootDirParams{WorkspacesRoot: params.WorkspacesRoot, WorkspaceID: params.WorkspaceID, WorkspaceSlug: params.WorkspaceSlug, TaskID: params.TaskID, IssueIdentifier: params.IssueIdentifier}) {
		t.Fatalf("retry environment = %#v, want the predicted root", env)
	}
}
