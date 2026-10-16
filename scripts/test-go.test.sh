#!/usr/bin/env bash
set -eu

# Runs the real test-go.sh, agent CLI guard and command name list against a
# throwaway Go module with one regular package and two pkg/agent packages.
# Each fixture test fails, invokes a guarded agent CLI name, or checks its
# race/parallel build on request, so every case asserts what the runner
# actually prints and returns. No product package, database or developer
# agent CLI is involved.
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TEST_DIR=$(mktemp -d "${TMPDIR:-/tmp}/multica-test-go.XXXXXX")
FIXTURE_DIR="$TEST_DIR/repo"
SERVER_DIR="$FIXTURE_DIR/server"
SHIM_DIR="$TEST_DIR/bin"
STDOUT_FILE="$TEST_DIR/stdout.log"
STDERR_FILE="$TEST_DIR/stderr.log"
MODULE=example.com/fixture/server

cleanup() {
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

if ! FIXTURE_REAL_GO=$(command -v go); then
  echo "test-go.test.sh: the go toolchain is required" >&2
  exit 1
fi
export FIXTURE_REAL_GO
export FIXTURE_GO_CALLS="$TEST_DIR/go-calls.log"

# Keep fixture builds offline, outside any workspace or user go env, and
# uncached so every case runs its tests.
export GOENV=off GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-count=1

mkdir -p "$FIXTURE_DIR/scripts" "$SHIM_DIR"
cp "$SCRIPT_DIR/test-go.sh" \
  "$SCRIPT_DIR/go-test-with-agent-cli-guard.sh" \
  "$SCRIPT_DIR/agent-cli-command-names.txt" \
  "$FIXTURE_DIR/scripts/"

# Records each go command for scope checks, then runs the real toolchain.
# FIXTURE_TERM_RUNNER=1 sends SIGTERM to the runner during its first go test.
cat >"$SHIM_DIR/go" <<'SHIM'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >>"$FIXTURE_GO_CALLS"
if [ "${FIXTURE_TERM_RUNNER:-}" = 1 ] && [ "${1:-}" = test ]; then
  # This shim's parent is the agent CLI guard; the guard's parent is the runner.
  runner=$(ps -o ppid= -p "$PPID" | tr -d ' ')
  case "$(ps -o args= -p "$runner")" in
    *scripts/test-go.sh*) kill -TERM "$runner" ;;
    *)
      echo "fixture go: test-go.sh is not the guard's parent" >&2
      exit 2
      ;;
  esac
fi
exec "$FIXTURE_REAL_GO" "$@"
SHIM
chmod 755 "$SHIM_DIR/go"

mkdir -p "$SERVER_DIR"
printf 'module %s\n\ngo 1.21\n' "$MODULE" >"$SERVER_DIR/go.mod"

# $1: package directory under server/; $2: Go package name; $3: suite half.
# FIXTURE_FAIL_<half>=1 fails the test with "<half> fixture failure";
# FIXTURE_AGENT_CLI_<half>=1 runs `claude` only if it resolves to the guard's
# sentinel; FIXTURE_EXPECT_RACE=1 requires a -race build. Agent tests require
# -parallel 2.
write_fixture_package() {
  dir="$SERVER_DIR/$1"
  mkdir -p "$dir"
  cat >"$dir/fixture_test.go" <<GO
package $2

import (
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const half = "$3"

func TestFixture(t *testing.T) {
	if os.Getenv("FIXTURE_EXPECT_RACE") == "1" && !raceEnabled {
		t.Fatal("built without -race")
	}
	if half == "agent" {
		if got := flag.Lookup("test.parallel").Value.String(); got != "2" {
			t.Fatalf("test.parallel = %s, want 2", got)
		}
	}
	if os.Getenv("FIXTURE_AGENT_CLI_"+half) == "1" {
		path, err := exec.LookPath("claude")
		if err != nil || !strings.Contains(path, "multica-agent-cli-guard.") {
			t.Fatalf("claude resolves outside the agent CLI guard: %q (%v)", path, err)
		}
		_ = exec.Command(path, "--version").Run()
	}
	if os.Getenv("FIXTURE_FAIL_"+half) == "1" {
		t.Fatalf("%s fixture failure", half)
	}
}
GO
  printf '//go:build race\n\npackage %s\n\nconst raceEnabled = true\n' "$2" >"$dir/race_on_test.go"
  printf '//go:build !race\n\npackage %s\n\nconst raceEnabled = false\n' "$2" >"$dir/race_off_test.go"
}

write_fixture_package internal/regular regular regular
write_fixture_package pkg/agent agent agent
write_fixture_package pkg/agent/sub sub agent

# $1: case label; then NAME=value fixture settings, `--`, and the test-go.sh
# arguments. Leaves stdout, stderr, the go call log and $status for the
# expect_* helpers.
run_case() {
  label=$1
  shift
  settings=()
  while [ "$1" != -- ]; do
    settings+=("$1")
    shift
  done
  shift
  : >"$FIXTURE_GO_CALLS"
  set +e
  env ${settings[@]+"${settings[@]}"} PATH="$SHIM_DIR:$PATH" \
    bash "$FIXTURE_DIR/scripts/test-go.sh" "$@" >"$STDOUT_FILE" 2>"$STDERR_FILE"
  status=$?
  set -e
}

fail_case() {
  {
    echo "$label: $1"
    echo "exit status: $status"
    echo "--- stdout"
    cat "$STDOUT_FILE"
    echo "--- stderr"
    cat "$STDERR_FILE"
    echo "--- go calls"
    cat "$FIXTURE_GO_CALLS"
  } >&2
  exit 1
}

expect_status() {
  if [ "$status" -ne "$1" ]; then
    fail_case "exit status $status, want $1"
  fi
}

# $1: stdout|stderr|calls; $2: extended regular expression.
expect_line() {
  if ! grep -Eq -- "$2" "$(case_file "$1")"; then
    fail_case "$1 lacks a line matching: $2"
  fi
}

reject_line() {
  if grep -Eq -- "$2" "$(case_file "$1")"; then
    fail_case "$1 has a line matching: $2"
  fi
}

case_file() {
  case "$1" in
    stdout) echo "$STDOUT_FILE" ;;
    stderr) echo "$STDERR_FILE" ;;
    calls) echo "$FIXTURE_GO_CALLS" ;;
  esac
}

# $1: package path under the module; $2: ok|FAIL. Native go test result line.
package_result() {
  echo "^$2[[:space:]]+$MODULE/$1([[:space:]]|\$)"
}

run_case "both halves pass" FIXTURE_EXPECT_RACE=1 -- --race
expect_status 0
expect_line stdout "$(package_result internal/regular ok)"
expect_line stdout "$(package_result pkg/agent ok)"
expect_line stdout "$(package_result pkg/agent/sub ok)"
expect_line stderr '^test-go: regular: passed$'
expect_line stderr '^test-go: agent: passed$'
expect_line calls '^test .*-p 2 .*\./pkg/agent/\.\.\.$'
reject_line stderr 'FAIL|incomplete|scoped'

run_case "regular fails" FIXTURE_FAIL_regular=1 --
expect_status 1
expect_line stdout 'regular fixture failure'
expect_line stdout "$(package_result internal/regular FAIL)"
expect_line stdout "$(package_result pkg/agent ok)"
expect_line stdout "$(package_result pkg/agent/sub ok)"
expect_line stderr '^test-go: regular: failed \(exit [1-9][0-9]*\)$'
expect_line stderr '^test-go: agent: passed$'
expect_line stderr '^test-go: FAIL \(regular\)$'

run_case "agent fails" FIXTURE_FAIL_agent=1 --
expect_status 1
expect_line stdout "$(package_result internal/regular ok)"
expect_line stdout 'agent fixture failure'
expect_line stdout "$(package_result pkg/agent FAIL)"
expect_line stdout "$(package_result pkg/agent/sub FAIL)"
expect_line stderr '^test-go: regular: passed$'
expect_line stderr '^test-go: agent: failed \(exit [1-9][0-9]*\)$'
expect_line stderr '^test-go: FAIL \(agent\)$'

run_case "both halves fail" FIXTURE_FAIL_regular=1 FIXTURE_FAIL_agent=1 --
expect_status 1
expect_line stdout "$(package_result internal/regular FAIL)"
expect_line stdout "$(package_result pkg/agent FAIL)"
expect_line stderr '^test-go: regular: failed \(exit [1-9][0-9]*\)$'
expect_line stderr '^test-go: agent: failed \(exit [1-9][0-9]*\)$'
expect_line stderr '^test-go: FAIL \(regular agent\)$'

run_case "--only regular" FIXTURE_FAIL_agent=1 -- --only regular
expect_status 0
expect_line stdout "$(package_result internal/regular ok)"
reject_line stdout "$MODULE/pkg/agent"
expect_line stderr '^test-go: scoped to the regular half'
expect_line stderr '^test-go: regular: passed$'
reject_line stderr '^test-go: agent:'
reject_line calls '\./pkg/agent/\.\.\.'

# Option order must not matter: CI spells it one way, humans another.
run_case "--only agent" FIXTURE_FAIL_regular=1 FIXTURE_EXPECT_RACE=1 -- --only agent --race
expect_status 0
expect_line stdout "$(package_result pkg/agent ok)"
expect_line stdout "$(package_result pkg/agent/sub ok)"
reject_line stdout "$MODULE/internal/regular"
expect_line stderr '^test-go: scoped to the agent half'
expect_line stderr '^test-go: agent: passed$'
reject_line stderr '^test-go: regular:'
reject_line calls '^list '

# Package arguments run only those packages; agent packages keep the throttle.
run_case "package arguments" -- "$MODULE/internal/regular" "$MODULE/pkg/agent/sub"
expect_status 0
expect_line stdout "$(package_result internal/regular ok)"
expect_line stdout "$(package_result pkg/agent/sub ok)"
reject_line stdout "$(package_result pkg/agent ok)"
expect_line calls "^test -p 2 -parallel 2 $MODULE/pkg/agent/sub\$"
reject_line calls '^list '

run_case "regular package only" FIXTURE_FAIL_agent=1 -- "$MODULE/internal/regular"
expect_status 0
expect_line stdout "$(package_result internal/regular ok)"
reject_line stdout "$MODULE/pkg/agent"
expect_line stderr '^test-go: regular: passed$'
reject_line stderr '^test-go: agent:'

run_case "agent CLI guard" FIXTURE_AGENT_CLI_regular=1 FIXTURE_AGENT_CLI_agent=1 --
expect_status 1
expect_line stderr '^unexpected agent CLI invocation: claude \[arguments redacted\]$'
expect_line stderr '^test-go: regular: failed \(exit [1-9][0-9]*\)$'
expect_line stderr '^test-go: agent: failed \(exit [1-9][0-9]*\)$'

# Two files with different package names make `go list ./...` fail.
mkdir -p "$SERVER_DIR/internal/broken"
printf 'package one\n' >"$SERVER_DIR/internal/broken/one.go"
printf 'package two\n' >"$SERVER_DIR/internal/broken/two.go"
run_case "package discovery fails" --
expect_status 1
expect_line stderr '^test-go: regular: incomplete \(package discovery failed, exit [1-9][0-9]*\)$'
reject_line stderr '^test-go: regular: passed'
reject_line calls "^test .*$MODULE/internal/regular"
expect_line stdout "$(package_result pkg/agent ok)"
expect_line stderr '^test-go: agent: passed$'
expect_line stderr '^test-go: FAIL \(regular\)$'
rm -rf "$SERVER_DIR/internal/broken"

mv "$SERVER_DIR/internal" "$TEST_DIR/internal"
run_case "package discovery finds no regular packages" --
expect_status 1
expect_line stderr '^test-go: regular: incomplete \(package discovery found no packages\)$'
reject_line stderr '^test-go: regular: passed'
expect_line stderr '^test-go: agent: passed$'
expect_line stderr '^test-go: FAIL \(regular\)$'
mv "$TEST_DIR/internal" "$SERVER_DIR/internal"

run_case "terminated during the regular half" FIXTURE_TERM_RUNNER=1 --
expect_status 143
reject_line stderr '^test-go: agent:'
reject_line stdout "$MODULE/pkg/agent"
reject_line calls '\./pkg/agent/\.\.\.'

# $1: case label; the rest are arguments for test-go.sh. Asserts the usage
# exit status, the usage line, and that go was never invoked.
expect_usage_failure() {
  run_case "$1" -- "${@:2}"
  expect_status 2
  if [ -s "$FIXTURE_GO_CALLS" ]; then
    fail_case "invoked go"
  fi
  expect_line stderr '^usage: .*test-go\.sh \[--race\] \[--only regular\|agent\] \[package \.\.\.\]$'
}

expect_usage_failure "unknown option" --unknown
expect_usage_failure "unknown --only scope" --only everything
expect_usage_failure "missing --only scope" --only

echo "test-go.test.sh: PASS"
