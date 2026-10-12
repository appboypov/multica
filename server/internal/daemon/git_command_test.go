package daemon

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestGitCommandContextPreservesFailureStderr(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	out, err := runGitCommandContext(context.Background(), root, 5*time.Second, "rev-parse", "--verify", "refs/heads/instruction-selection-missing-ref")
	if err == nil {
		t.Fatal("missing reference unexpectedly resolved")
	}
	if !strings.Contains(out, "fatal:") || strings.TrimSpace(out) != out {
		t.Fatalf("text helper output = %q, want trimmed git stderr", out)
	}
	raw, err := runGitCommandRawContext(context.Background(), root, 5*time.Second, "rev-parse", "--verify", "refs/heads/instruction-selection-missing-ref")
	if err == nil || raw != "" {
		t.Fatalf("stdout-only helper = %q, %v; want no stderr in output", raw, err)
	}
}
