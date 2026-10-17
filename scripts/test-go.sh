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
# selects one half so CI can give each its own runner; the default still runs
# both for `make test`, check.sh, and the release workflow.
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

cd "$REPO_ROOT/server"

# Package arguments limit the run to those import paths; without them every
# package runs.
if [ "${#selected[@]}" -gt 0 ]; then
  packages=$(printf '%s\n' "${selected[@]}")
else
  packages=$(go list ./...)
fi
regular_packages=()
agent_packages=()
for package in $packages; do
  case "$package" in
    */pkg/agent|*/pkg/agent/*) agent_packages+=("$package") ;;
    *) regular_packages+=("$package") ;;
  esac
done
[ "${#selected[@]}" -gt 0 ] || agent_packages=(./pkg/agent/...)

if [ "$only" != agent ] && [ "${#regular_packages[@]}" -gt 0 ]; then
  "$GUARD_SCRIPT" -- go "${go_test_args[@]}" "${regular_packages[@]}"
fi

if [ "$only" != regular ] && [ "${#agent_packages[@]}" -gt 0 ]; then
  # Subprocess-backed agent tests have hard deadlines. Limit both package and
  # within-package parallelism so race builds do not starve their parent loops.
  "$GUARD_SCRIPT" -- go "${go_test_args[@]}" -p 2 -parallel 2 "${agent_packages[@]}"
fi
