#!/usr/bin/env bash
# Runs inside the full-check container that scripts/full-check.sh starts.
#
# Usage: full-check-container.sh [changed-file ...]
#
# The arguments are the repo-relative files the checkout changes against
# origin/main. Unit tests then run only for test files that import one of
# them. Without arguments, or when a package manifest, lockfile, TypeScript
# or Vitest config, or test setup file changed, every unit test runs.
#
# Every stage runs to its end and records its exit status; a stage that fails
# fails the check, and a check that runs past its 10-minute budget is flagged
# as slow. TURBO_API, TURBO_TOKEN and TURBO_TEAM name the Turborepo cache
# server on the Mac, so a package whose
# inputs are unchanged replays its earlier result, and the Go stage is skipped
# when its inputs match a run that passed. Ports and the database reach only
# the stages that start servers: turbo hashes them, so exporting them
# everywhere would change every task's key.
set -uo pipefail
set -m # each stage runs in its own process group, so its end stops what it left running

: "${TURBO_API:?}" "${TURBO_TOKEN:?}" "${TURBO_TEAM:?}"
BUDGET_SEC=${BUDGET_SEC:-600}
PGBIN=/usr/lib/postgresql/17/bin
CHANGED=("$@")
START=$SECONDS

RUN=$(mktemp -d /tmp/multica-full-check.XXXXXX)
cleanup() {
  [ -f "$RUN/pg/postmaster.pid" ] && "$PGBIN/pg_ctl" -D "$RUN/pg" -m immediate -w stop >/dev/null
  rm -rf "$RUN"
}
trap cleanup EXIT

export DO_NOT_TRACK=1 NEXT_TELEMETRY_DISABLED=1 TURBO_TELEMETRY_DISABLED=1
export PLAYWRIGHT_BROWSERS_PATH=/home/crabbox/.cache/ms-playwright COREPACK_HOME=/usr/local/share/corepack

RESULTS=()
FAILED=0
LAST_RC=0
stage() {
  local name=$1
  shift
  echo
  echo "==> stage $name"
  local start=$SECONDS
  "$@" &
  local pid=$!
  wait "$pid"
  LAST_RC=$?
  kill -TERM -- "-$pid" 2>/dev/null
  [ "$LAST_RC" -eq 0 ] || FAILED=1
  RESULTS+=("$name=$LAST_RC ($((SECONDS - start))s)")
  echo "==> stage $name exit $LAST_RC"
}

read -r PGPORT BACKPORT WEBPORT < <(python3 -c 'import socket; ss=[socket.socket() for _ in range(3)]; [s.bind(("127.0.0.1",0)) for s in ss]; print(*[s.getsockname()[1] for s in ss])')
with_services_env() {
  env DATABASE_URL="postgres://multica@localhost:$PGPORT/multica?sslmode=disable" \
    PORT="$BACKPORT" FRONTEND_PORT="$WEBPORT" FRONTEND_ORIGIN="http://localhost:$WEBPORT" \
    NEXT_PUBLIC_API_URL="http://localhost:$BACKPORT" NEXT_PUBLIC_WS_URL="ws://localhost:$BACKPORT/ws" \
    MULTICA_SERVER_URL="ws://localhost:$BACKPORT/ws" PLAYWRIGHT_BASE_URL="http://localhost:$WEBPORT" \
    APP_ENV=development "$@"
}

all_unit_tests() {
  [ ${#CHANGED[@]} -eq 0 ] && return 0
  local f
  for f in "${CHANGED[@]}"; do
    case "$f" in
      package.json | */package.json | pnpm-lock.yaml | pnpm-workspace.yaml | turbo.json | \
        tsconfig*.json | */tsconfig*.json | */vitest.config.* | */vite.config.* | */test/setup.*) return 0 ;;
    esac
  done
  return 1
}

related_unit_tests() {
  local files=() dirs=() f dir rc=0
  for f in "${CHANGED[@]}"; do
    [ -f "$f" ] && files+=("$PWD/$f")
  done
  [ ${#files[@]} -gt 0 ] || return 0
  # Every workspace package whose tests run with Vitest, apart from mobile.
  while IFS= read -r dir; do dirs+=("$dir"); done < <(node -e '
    const fs = require("fs");
    for (const dir of process.argv.slice(1)) {
      if (dir === "apps/mobile" || !fs.existsSync(`${dir}/package.json`)) continue;
      const test = JSON.parse(fs.readFileSync(`${dir}/package.json`, "utf8")).scripts?.test ?? "";
      if (test.startsWith("vitest")) console.log(dir);
    }' packages/* apps/*)
  for dir in "${dirs[@]}"; do
    echo "--- unit tests in $dir that import a changed file"
    (cd "$dir" && pnpm exec vitest related --run --passWithNoTests "${files[@]}") || rc=1
  done
  return "$rc"
}

go_inputs_key() {
  {
    go env GOVERSION
    "$PGBIN/postgres" --version
    find server scripts/test-go.sh scripts/test-go.test.sh scripts/go-test-with-agent-cli-guard.sh \
      scripts/agent-cli-command-names.txt -type f -print0 | sort -z | xargs -0 sha256sum
  } | sha256sum | cut -d' ' -f1
}

# The Go pass marker is an empty artifact on the Turborepo cache server.
go_passed() {
  curl -sf -o /dev/null -H "Authorization: Bearer $TURBO_TOKEN" "$TURBO_API/v8/artifacts/$1?slug=$TURBO_TEAM"
}
record_go_pass() {
  curl -sf -o /dev/null -X PUT -H "Authorization: Bearer $TURBO_TOKEN" -H "Content-Type: application/octet-stream" \
    --data-binary '' "$TURBO_API/v8/artifacts/$1?slug=$TURBO_TEAM"
}

# The e2e stage needs the schema even when the Go stage is skipped.
migrate_stage() {
  (cd server && with_services_env go run ./cmd/migrate up)
}

go_stage() {
  bash scripts/test-go.test.sh &&
    with_services_env bash scripts/test-go.sh &&
    (cd server && go vet ./...)
}

e2e_stage() {
  (cd server && go build -o "$RUN/server" ./cmd/server) || return
  (cd server && with_services_env "$RUN/server") >"$RUN/backend.log" 2>&1 &
  with_services_env pnpm dev:web >"$RUN/frontend.log" 2>&1 &
  until curl -sf "http://localhost:$BACKPORT/health" >/dev/null; do sleep 1; done
  until curl -sf "http://localhost:$WEBPORT/" >/dev/null; do sleep 1; done
  with_services_env pnpm exec playwright test --reporter=line
}

"$PGBIN/initdb" -D "$RUN/pg" -U multica --auth=trust >/dev/null
"$PGBIN/pg_ctl" -D "$RUN/pg" -o "-p $PGPORT -k $RUN -c listen_addresses=127.0.0.1 -c max_connections=200 -c fsync=off" \
  -l "$RUN/pg.log" -w start >/dev/null
"$PGBIN/createdb" -h "$RUN" -p "$PGPORT" -U multica multica

stage install pnpm install --frozen-lockfile --filter='!@multica/mobile'
TURBO=(pnpm exec turbo run --filter='!@multica/mobile' --continue --concurrency=3 --output-logs=new-only)
if all_unit_tests; then
  stage turbo "${TURBO[@]}" typecheck lint test
else
  stage turbo "${TURBO[@]}" typecheck lint
  stage unit related_unit_tests
fi
stage ui-exports pnpm check:ui-exports
stage ui-radii pnpm check:ui-radii

stage migrate migrate_stage
GO_KEY=$(go_inputs_key)
if go_passed "$GO_KEY"; then
  RESULTS+=("go=skipped (same inputs passed before)")
else
  stage go go_stage
  [ "$LAST_RC" -ne 0 ] || record_go_pass "$GO_KEY"
fi

stage e2e e2e_stage
stage madspec-sync uv run --with pytest --with pyyaml pytest -p no:cacheprovider -q madspec-sync/tests

echo
echo "full-check summary ($((SECONDS - START))s of ${BUDGET_SEC}s budget)"
for r in "${RESULTS[@]}"; do echo "  $r"; done
[ $((SECONDS - START)) -le "$BUDGET_SEC" ] || echo "  slow: over the ${BUDGET_SEC}s budget"
exit "$FAILED"
