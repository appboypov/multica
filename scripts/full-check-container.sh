#!/usr/bin/env bash
# Runs inside the full-check container that scripts/full-check.sh starts.
#
# Usage: full-check-container.sh [changed-file ...]
#
# The arguments are the repo-relative files the checkout changes against
# origin/main. Unit tests run when they import a changed file or read it as a
# filesystem input declared in related_unit_tests. Go tests run only for the
# changed packages and their importers. E2e runs only for specs whose first
# name word starts a changed path segment. Madspec-sync runs when it changes.
# Without arguments, as on main, everything runs. A package manifest or
# lockfile change runs every unit test and every e2e spec, as do the changes
# named in all_unit_tests, go_packages and e2e_specs.
#
# Every stage runs to its end and records its exit status; a stage that fails
# fails the check, and a check that runs past its 10-minute budget is flagged
# as slow. E2e server readiness has a shared 120-second startup deadline.
# A server exit or failed readiness prints both server logs and fails only e2e.
# TURBO_API, TURBO_TOKEN and TURBO_TEAM name the Turborepo cache server on
# the Mac, so a package whose
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
    # Filesystem inputs do not appear in Vitest's import graph. Add their
    # reader tests as related roots, including when an input was deleted.
    case "$f" in
      apps/desktop/electron-builder.yml)
        files+=("$PWD/apps/desktop/scripts/package.test.mjs") ;;
      packages/views/locales/*/*.json)
        files+=("$PWD/packages/views/locales/parity.test.ts") ;;
    esac
    case "$f" in
      packages/ui/* | packages/views/* | apps/web/* | apps/desktop/src/*)
        case "$f" in
          *.ts | *.tsx | *.css)
            files+=("$PWD/apps/web/app/text-contrast.test.ts") ;;
        esac ;;
    esac
    case "$f" in
      packages/ui/* | packages/core/* | packages/views/* | apps/web/* | apps/desktop/src/*)
        case "$f" in
          *.ts | *.tsx | *.css)
            files+=("$PWD/apps/web/app/brand-variant-cascade.test.ts") ;;
        esac ;;
    esac
    # Export validity also depends on target files existing.
    case "$f" in
      packages/* | apps/*)
        files+=("$PWD/packages/views/rich-content/package-exports.test.ts") ;;
    esac
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
    echo "--- unit tests in $dir that import or read a changed file"
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

# Prints the Go import paths whose tests the change reaches: the packages of
# the changed files under server/ and every package that imports them, also
# from its tests. Prints "all" when every package runs and nothing when the
# change touches no Go.
go_packages() {
  [ ${#CHANGED[@]} -eq 0 ] && { echo all; return; }
  local f dir dirs=()
  for f in "${CHANGED[@]}"; do
    case "$f" in
      server/go.mod | server/go.sum | server/migrations/* | scripts/test-go.sh | scripts/test-go.test.sh | \
        scripts/go-test-with-agent-cli-guard.sh | scripts/agent-cli-command-names.txt) echo all; return ;;
      server/*) ;;
      *) continue ;;
    esac
    # A file with no Go code beside it, such as an embedded asset, belongs to
    # the nearest folder above it that holds Go code.
    dir=$(dirname "$f")
    while [ "$dir" != server ] && ! compgen -G "$dir/*.go" >/dev/null; do dir=$(dirname "$dir"); done
    [ "$dir" != server ] && dirs+=("./${dir#server/}")
  done
  [ ${#dirs[@]} -gt 0 ] || return 0
  (
    cd server &&
      go list -f '{{.ImportPath}} {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... |
      awk -v changed="$(go list "${dirs[@]}" | tr '\n' ' ')" '
        BEGIN { n = split(changed, c, " "); for (i = 1; i <= n; i++) hit[c[i]] = 1 }
        { for (i = 1; i <= NF; i++) if ($i in hit) { print $1; next } }'
  )
}

# Prints the e2e specs the change reaches, or "all".
e2e_specs() {
  [ ${#CHANGED[@]} -eq 0 ] && { echo all; return; }
  local f spec word seg
  for f in "${CHANGED[@]}"; do
    case "$f" in
      e2e/*.spec.ts) ;;
      e2e/* | playwright.config.* | package.json | */package.json | pnpm-lock.yaml | pnpm-workspace.yaml) echo all; return ;;
    esac
  done
  for spec in e2e/*.spec.ts; do
    word=$(basename "$spec")
    word=${word%%[-.]*}
    for f in "${CHANGED[@]}"; do
      [ "$f" = "$spec" ] && { echo "$spec"; continue 2; }
      case "$f" in e2e/*) continue ;; esac
      for seg in ${f//\// }; do
        case "$seg" in "$word"*) echo "$spec"; continue 3 ;; esac
      done
    done
  done
}

madspec_sync_changed() {
  [ ${#CHANGED[@]} -eq 0 ] && return 0
  local f
  for f in "${CHANGED[@]}"; do
    case "$f" in madspec-sync/*) return 0 ;; esac
  done
  return 1
}

migrate_stage() {
  (cd server && with_services_env go run ./cmd/migrate up)
}

go_stage() {
  if [ "$GO_PACKAGES" = all ]; then
    bash scripts/test-go.test.sh &&
      with_services_env bash scripts/test-go.sh &&
      (cd server && go vet ./...)
  else
    # shellcheck disable=SC2086 # one import path per word
    with_services_env bash scripts/test-go.sh $GO_PACKAGES &&
      (cd server && go vet $GO_PACKAGES)
  fi
}

wait_e2e_servers() {
  local backend_pid=$1 frontend_pid=$2 deadline=$((SECONDS + 120))
  while :; do
    if ! kill -0 "$backend_pid" 2>/dev/null; then
      echo "full-check: backend exited before e2e readiness" >&2
      break
    fi
    if ! kill -0 "$frontend_pid" 2>/dev/null; then
      echo "full-check: frontend exited before e2e readiness" >&2
      break
    fi
    if [ "$SECONDS" -ge "$deadline" ]; then
      echo "full-check: e2e servers not ready within 120 seconds" >&2
      break
    fi
    if curl -sf --connect-timeout 1 --max-time 2 "http://localhost:$BACKPORT/health" >/dev/null &&
      curl -sf --connect-timeout 1 --max-time 2 "http://localhost:$WEBPORT/" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "--- backend startup log" >&2
  cat "$RUN/backend.log" >&2
  echo "--- frontend startup log" >&2
  cat "$RUN/frontend.log" >&2
  return 1
}

e2e_stage() {
  (cd server && go build -o "$RUN/server" ./cmd/server) || return
  (cd server && with_services_env "$RUN/server") >"$RUN/backend.log" 2>&1 &
  local backend_pid=$!
  with_services_env pnpm dev:web >"$RUN/frontend.log" 2>&1 &
  local frontend_pid=$!
  wait_e2e_servers "$backend_pid" "$frontend_pid" || return 1
  if [ "$E2E_SPECS" = all ]; then
    with_services_env pnpm exec playwright test --reporter=line
  else
    # shellcheck disable=SC2086 # one spec path per word
    with_services_env pnpm exec playwright test --reporter=line $E2E_SPECS
  fi
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

GO_PACKAGES=$(go_packages | tr '\n' ' ')
GO_PACKAGES=${GO_PACKAGES% }
E2E_SPECS=$(e2e_specs | tr '\n' ' ')
E2E_SPECS=${E2E_SPECS% }
GO_KEY=$(go_inputs_key)
GO_CACHED=0
[ "$GO_PACKAGES" = all ] && go_passed "$GO_KEY" && GO_CACHED=1

if { [ -n "$GO_PACKAGES" ] && [ "$GO_CACHED" -eq 0 ]; } || [ -n "$E2E_SPECS" ]; then
  stage migrate migrate_stage
fi

if [ -z "$GO_PACKAGES" ]; then
  RESULTS+=("go=skipped (no Go change)")
elif [ "$GO_CACHED" -eq 1 ]; then
  RESULTS+=("go=skipped (same inputs passed before)")
else
  echo "full-check: Go packages: $GO_PACKAGES"
  stage go go_stage
  # Only a run of every package marks the inputs as passed, so main still
  # runs them all after a merge.
  [ "$LAST_RC" -ne 0 ] || [ "$GO_PACKAGES" != all ] || record_go_pass "$GO_KEY"
fi

if [ -z "$E2E_SPECS" ]; then
  RESULTS+=("e2e=skipped (no spec reaches the change)")
else
  echo "full-check: e2e specs: $E2E_SPECS"
  stage e2e e2e_stage
fi

if madspec_sync_changed; then
  stage madspec-sync uv run --with pytest --with pyyaml pytest -p no:cacheprovider -q madspec-sync/tests
else
  RESULTS+=("madspec-sync=skipped (not changed)")
fi

echo
echo "full-check summary ($((SECONDS - START))s of ${BUDGET_SEC}s budget)"
for r in "${RESULTS[@]}"; do echo "  $r"; done
[ $((SECONDS - START)) -le "$BUDGET_SEC" ] || echo "  slow: over the ${BUDGET_SEC}s budget"
exit "$FAILED"
