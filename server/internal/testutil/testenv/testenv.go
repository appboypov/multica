// Package testenv gives a test binary the same environment wherever it is
// started: no daemon task identity, no daemon task marker above the working
// directory, and no git settings beyond the ones a fixture writes. Call these
// from TestMain, before m.Run, so parallel tests see the same environment as
// sequential ones.
//
// The package must only be imported from _test files.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
)

// daemonTaskEnv lists every variable the daemon sets for a task it runs (see
// taskMulticaEnvironment and runTask's agentEnv in internal/daemon). Each one
// switches the CLI and the daemon into their task-scoped behaviour.
var daemonTaskEnv = []string{
	"MULTICA_TOKEN",
	"MULTICA_TASK_CONFIG_ROOT",
	"MULTICA_TASK_WORKSPACES_ROOT",
	"MULTICA_SERVER_URL",
	"MULTICA_DAEMON_PORT",
	"MULTICA_WORKSPACE_ID",
	"MULTICA_AGENT_NAME",
	"MULTICA_AGENT_ID",
	"MULTICA_TASK_ID",
	"MULTICA_TASK_SLOT",
	"MULTICA_AGENT_QUESTIONS",
	"MULTICA_AUTOPILOT_RUN_ID",
	"MULTICA_AUTOPILOT_ID",
	"MULTICA_QUICK_CREATE_TASK_ID",
	"MULTICA_QUICK_CREATE_ATTACHMENT_IDS",
	"MULTICA_DSH_SESSION_ROOT",
}

// ClearDaemonTaskEnv unsets the variables a daemon-managed task carries, so
// identity, config and workspace resolve outside task scope. A test that
// wants the task-scoped branch sets the variables it needs itself.
func ClearDaemonTaskEnv() {
	for _, key := range daemonTaskEnv {
		os.Unsetenv(key)
	}
}

// LeaveDaemonTaskDir moves the process into a fresh directory under the system
// temp directory, which no daemon workspaces root contains, so the CLI's
// upward search for a daemon task marker finds none. It returns a function
// that removes the directory.
func LeaveDaemonTaskDir() (cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "multica-test-cwd-")
	if err != nil {
		return nil, fmt.Errorf("create test working directory: %w", err)
	}
	if err := os.Chdir(dir); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("enter test working directory: %w", err)
	}
	return func() {
		// Windows cannot remove the current directory; step out first.
		os.Chdir(os.TempDir())
		os.RemoveAll(dir)
	}, nil
}

// IsolateGitConfig points git's global config at an empty file, skips the
// system config and drops config passed through the environment, and returns
// a function that removes the file. Fixtures supply their own identity; a test
// that needs a global setting writes it to GIT_CONFIG_GLOBAL itself.
func IsolateGitConfig() (cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "multica-test-git-")
	if err != nil {
		return nil, fmt.Errorf("create isolated git config directory: %w", err)
	}
	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(global, nil, 0o644); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("create isolated git config: %w", err)
	}
	os.Setenv("GIT_CONFIG_GLOBAL", global)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Unsetenv("GIT_CONFIG_COUNT")
	os.Unsetenv("GIT_CONFIG_PARAMETERS")
	return func() { os.RemoveAll(dir) }, nil
}
