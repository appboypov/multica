#!/usr/bin/env bash
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
GUARD_SCRIPT="$SCRIPT_DIR/go-test-with-agent-cli-guard.sh"

usage() {
  echo "usage: $0 [--race] [--only regular|agent] [package ...]" >&2
}

# The suite is two `go test` invocations: every package outside pkg/agent at
# the default parallelism, then pkg/agent throttled (see below). `--only`
# selects one half so CI can give each its own runner; a scoped run is not a
# complete suite result. The default runs both for `make test`, check.sh, and
# the release workflow, and runs the agent half even when the regular half
# fails, so one half's failure never hides the other's result.
#
# Each half's raw output passes through unchanged. This script reports each
# half's result on stderr and exits 1 when any half failed or could not run: a
# package discovery failure is an incomplete half, never an empty pass.
go_test_args=(test)
only=all
selected=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --race)
      go_test_args+=(-race)
      shift
      ;;
    --only)
      case "${2:-}" in
        regular|agent) only=$2 ;;
        *)
          usage
          exit 2
          ;;
      esac
      shift 2
      ;;
    -*)
      usage
      exit 2
      ;;
    *)
      selected+=("$1")
      shift
      ;;
  esac
done

# A failed half no longer stops the suite, but an interrupt still does: bash
# runs these traps once the running invocation returns, before the next half.
trap 'exit 130' INT
trap 'exit 143' TERM

failed_halves=()

# $1: half; $2: its result. Anything but "passed" fails the suite.
report() {
  echo "test-go: $1: $2" >&2
  if [ "$2" != passed ]; then
    failed_halves+=("$1")
  fi
}

# $1: half; the rest are its `go test` flags and packages.
run_half() {
  half=$1
  shift
  set +e
  "$GUARD_SCRIPT" -- go "${go_test_args[@]}" "$@"
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    report "$half" passed
  else
    report "$half" "failed (exit $status)"
  fi
}

cd "$REPO_ROOT/server"

if [ "$only" != all ]; then
  echo "test-go: scoped to the $only half (--only); not a complete suite result" >&2
fi

# Package arguments limit the run to those import paths; without them every
# package runs.
list_status=0
packages=
if [ "${#selected[@]}" -gt 0 ]; then
  packages=$(printf '%s\n' "${selected[@]}")
elif [ "$only" != agent ]; then
  set +e
  packages=$(go list ./...)
  list_status=$?
  set -e
fi
regular_packages=()
agent_packages=()
if [ "$list_status" -eq 0 ]; then
  for package in $packages; do
    case "$package" in
      */pkg/agent|*/pkg/agent/*) agent_packages+=("$package") ;;
      *) regular_packages+=("$package") ;;
    esac
  done
fi
[ "${#selected[@]}" -gt 0 ] || agent_packages=(./pkg/agent/...)

if [ "$only" != agent ]; then
  if [ "$list_status" -ne 0 ]; then
    report regular "incomplete (package discovery failed, exit $list_status)"
  elif [ "${#regular_packages[@]}" -gt 0 ]; then
    run_half regular "${regular_packages[@]}"
  elif [ "${#selected[@]}" -eq 0 ]; then
    report regular "incomplete (package discovery found no packages)"
  fi
fi

if [ "$only" != regular ] && [ "${#agent_packages[@]}" -gt 0 ]; then
  # Subprocess-backed agent tests have hard deadlines. Limit both package and
  # within-package parallelism so race builds do not starve their parent loops.
  run_half agent -p 2 -parallel 2 "${agent_packages[@]}"
fi

if [ "${#failed_halves[@]}" -gt 0 ]; then
  echo "test-go: FAIL (${failed_halves[*]})" >&2
  exit 1
fi
