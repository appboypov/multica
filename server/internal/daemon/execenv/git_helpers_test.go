package execenv

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func worktreeTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testRepoTemplate is the repository newTestRepo hands out copies of. It is
// built once per test binary: copying it spawns no git process, where
// building each repo afresh spawned five, a sizeable share of a single-turn
// test. TestMain removes it.
var testRepoTemplate struct {
	once sync.Once
	dir  string
	err  error
}

// newTestRepo creates a git repo with one commit and returns its path. The
// repo carries its own identity config so tests don't depend on the machine's
// global git setup.
func newTestRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	testRepoTemplate.once.Do(func() {
		testRepoTemplate.dir, testRepoTemplate.err = buildTestRepoTemplate()
	})
	if testRepoTemplate.err != nil {
		t.Fatalf("build the template repo: %v", testRepoTemplate.err)
	}
	dir := t.TempDir()
	// macOS hands out /var/folders temp dirs that are symlinks to /private/var.
	// resolveGitRoot canonicalises, so the test must compare against the
	// canonical form too.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	if err := os.CopyFS(dir, os.DirFS(testRepoTemplate.dir)); err != nil {
		t.Fatalf("copy the template repo: %v", err)
	}
	return dir
}

func buildTestRepoTemplate() (string, error) {
	dir, err := os.MkdirTemp("", "multica-test-repo")
	if err != nil {
		return "", err
	}
	for name, content := range map[string]string{"tracked.txt": "original\n", "keep.txt": "keep\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return dir, err
		}
	}
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.name", "Test User"},
		{"config", "user.email", "test@test.com"},
		// git commit forks a DETACHED `git maintenance run --auto` and
		// returns without waiting for it. That background process creates
		// and removes lock files under .git/ after this function is done,
		// so newTestRepo's os.CopyFS can list .git/objects/maintenance.lock
		// and then fail to open it — failing whichever test happened to copy
		// the template at that moment, in a package that has nothing to do
		// with git maintenance. Every copy inherits this config, which also
		// keeps the git commands tests run on their own copy from spawning a
		// background process that races the assertions against it.
		{"config", "gc.auto", "0"},
		{"config", "maintenance.auto", "false"},
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			return dir, fmt.Errorf("git %v: %s: %w", args, out, err)
		}
	}
	return dir, nil
}

// requireGit skips the whole test when git is missing from the environment.
// This is the ONLY place a skip is allowed: once a test is past it, every git
// command is part of the assertion surface, so a failure there is a real
// regression and must fail loudly. An earlier version skipped on any git error,
// which would have turned "the delivered branch is gone" into a green run.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is not available: %v", err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s: %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}

func gitTry(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
