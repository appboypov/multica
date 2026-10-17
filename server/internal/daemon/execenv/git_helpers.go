package execenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const gitTimeout = 2 * time.Minute

// runGit runs git in dir and returns combined output. Callers inspect the
// output for git's own error text, so stdout and stderr stay merged.
func runGit(dir string, args ...string) (string, error) {
	return runGitEnv(dir, nil, args...)
}

// runGitEnv is runGit with extra environment entries, for the one caller that
// has to redirect GIT_INDEX_FILE.
func runGitEnv(dir string, extraEnv []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runGitTrimmed runs git for its stdout value, discarding stderr so a
// diagnostic line can't be mistaken for the value (`rev-parse` output, a
// config value, a stash sha).
func runGitTrimmed(dir string, args ...string) (string, error) {
	out, err := runGitStdout(dir, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// runGitTrimmedEnv is runGitTrimmed with extra environment entries.
func runGitTrimmedEnv(dir string, extraEnv []string, args ...string) (string, error) {
	out, err := runGitEnv(dir, extraEnv, args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(out), err)
	}
	return strings.TrimSpace(out), nil
}

// runGitStdout is runGitTrimmed without the trimming, for output where
// whitespace is significant — NUL-separated file listings, where a leading or
// trailing space is part of a filename.
func runGitStdout(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		return "", withGitStderr(err)
	}
	return string(out), nil
}

// withGitStderr renders git's stderr into the error.
//
// cmd.Output() already captures stderr into ExitError.Stderr, but nothing ever
// read it back out, so every failure from this path reached the user as a bare
// "exit status 1" — git's own explanation was collected and then thrown away
// at the point it was needed. Discarding stderr from the RESULT is deliberate
// (see runGitTrimmed: a diagnostic line must never be mistaken for a value);
// discarding it from the error was not.
func withGitStderr(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
			return fmt.Errorf("%s: %w", msg, err)
		}
	}
	return err
}
