//go:build !windows

package processtree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCombinedOutputKillsDescendantsHoldingOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	marker := filepath.Join(dir, "descendant-survived")
	script := filepath.Join(dir, "helper.sh")
	body := "#!/bin/sh\n(sleep 0.5; echo leaked > \"$1\") &\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := CombinedOutput(ctx, exec.Command(script, marker), 250*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CombinedOutput error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("CombinedOutput took %s, want a hard bounded return", elapsed)
	}

	time.Sleep(600 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("descendant survived process-tree cancellation, stat error = %v", err)
	}
}

func TestCombinedOutputKillsDescendantsAfterLeaderExit(t *testing.T) {
	t.Parallel()

	marker := filepath.Join(t.TempDir(), "descendant-survived")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	cmd := exec.Command("/bin/sh", "-c", `(sleep 0.5; echo leaked > "$1") >/dev/null 2>&1 & exit 0`, "helper", marker)
	if _, err := CombinedOutput(ctx, cmd, 250*time.Millisecond); err != nil {
		t.Fatalf("CombinedOutput after leader exit: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("CombinedOutput took %s, want bounded descendant cleanup", elapsed)
	}
	time.Sleep(600 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("descendant survived leader exit, stat error = %v", err)
	}
}
