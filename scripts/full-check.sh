#!/usr/bin/env bash
# The full check: run it on the Mac from this checkout after make local-release
# installed the head the release takes. A release never waits for it.
#
# It runs scripts/full-check-container.sh in a single-use local Crabbox
# container (MadSpec's crabbox-local), with the image that
# scripts/full-check.Dockerfile describes. Full checks on this Mac queue behind
# each other and share one Turborepo cache server, the multica-turbo-cache
# container on 127.0.0.1:41230. Unit tests run only for the files this
# checkout changes against origin/main, so on main every unit test runs.
# .crabbox.yaml keeps node_modules, .env and .env.worktree out of the container.
set -euo pipefail

STATE="$HOME/.local/state/multica-full-check"
CACHE_CONTAINER=multica-turbo-cache
CACHE_PORT=41230
CRABBOX_LOCAL=${CRABBOX_LOCAL:-$HOME/.agents/skills/madspec-remote-checks-and-tests/scripts/crabbox-local}

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"
mkdir -p "$STATE"
chmod 700 "$STATE"
if [ -z "${MULTICA_FULL_CHECK_QUEUED:-}" ]; then
  status=0
  MULTICA_FULL_CHECK_QUEUED=1 lockf -ks -t 0 "$STATE/queue.lock" "$0" "$@" || status=$?
  # lockf exits 75 when another full check holds the queue.
  if [ "$status" -eq 75 ]; then
    echo "full-check: waiting for the full-check queue"
    exec env MULTICA_FULL_CHECK_QUEUED=1 lockf -k "$STATE/queue.lock" "$0" "$@"
  fi
  exit "$status"
fi

git fetch -q origin
# Crabbox syncs the working tree with its untracked files, so their changes count too.
CHANGED=()
while IFS= read -r f; do CHANGED+=("$f"); done < <(
  git diff --name-only "$(git merge-base origin/main HEAD)"
  git ls-files --others --exclude-standard
)

[ -s "$STATE/turbo-token" ] || (umask 077 && openssl rand -hex 32 >"$STATE/turbo-token")
TOKEN=$(cat "$STATE/turbo-token")
if ! docker container inspect "$CACHE_CONTAINER" >/dev/null 2>&1; then
  docker run -d --name "$CACHE_CONTAINER" --restart unless-stopped -p "127.0.0.1:$CACHE_PORT:3000" \
    -e TURBO_TOKEN="$TOKEN" -e STORAGE_PROVIDER=local -e STORAGE_PATH=turborepocache \
    -v "$STATE/turbo-cache:/tmp/turborepocache" ducktors/turborepo-remote-cache:2.14.3 >/dev/null
fi
docker start "$CACHE_CONTAINER" >/dev/null

IMAGE_INPUTS=(scripts/full-check.Dockerfile pnpm-lock.yaml pnpm-workspace.yaml .npmrc server/go.mod server/go.sum)
IMAGE="madspec-crabbox-local:multica-full-check-$(cat "${IMAGE_INPUTS[@]}" | shasum -a 256 | cut -c1-12)"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  docker image inspect madspec-crabbox-local:1 >/dev/null 2>&1 || "$CRABBOX_LOCAL" build-image
  tar -c "${IMAGE_INPUTS[@]}" | docker build -t "$IMAGE" -f scripts/full-check.Dockerfile -
fi

echo "full-check: $(git rev-parse --short HEAD), ${#CHANGED[@]} changed files against origin/main"
exec "$CRABBOX_LOCAL" run --image "$(docker image inspect -f '{{.Id}}' "$IMAGE")" --cpus 6 --memory 12g --reason isolated -- \
  env TURBO_API="http://host.docker.internal:$CACHE_PORT" TURBO_TEAM=multica TURBO_TOKEN="$TOKEN" \
  bash scripts/full-check-container.sh ${CHANGED[@]+"${CHANGED[@]}"}
