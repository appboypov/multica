#!/usr/bin/env bash
# Merges the latest multica-ai/multica release into this fork's main, keeps the
# fork free of upstream's pipeline files, and runs the checks a merge must pass.
# Usage: bash scripts/upstream-sync.sh
set -euo pipefail

UPSTREAM_REPO="multica-ai/multica"
FORK_REMOVED_PATHS=(
  .github/workflows
  .github/actions
  .github/ci-paths.json
  .github/RELEASING.md
  .goreleaser.yml
  .vercelignore
  scripts/ci-scope.mjs
  scripts/ci-scope.test.mjs
)

if [ "$(git branch --show-current)" != "main" ]; then
  echo "upstream-sync runs on main" >&2
  exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "Commit or finish the open work before syncing" >&2
  exit 1
fi

tag="$(gh release view --repo "$UPSTREAM_REPO" --json tagName --jq .tagName)"
echo "==> Latest $UPSTREAM_REPO release: $tag"
git fetch --no-tags upstream tag "$tag"

if git merge-base --is-ancestor "$tag^{commit}" HEAD; then
  echo "==> main already contains $tag"
else
  echo "==> Merging $tag into main..."
  if ! git merge --no-ff --no-commit "$tag"; then
    git rev-parse -q --verify MERGE_HEAD > /dev/null || exit 1
  fi
  git rm -r -q --ignore-unmatch -- "${FORK_REMOVED_PATHS[@]}"
  conflicts="$(git diff --name-only --diff-filter=U)"
  if [ -n "$conflicts" ]; then
    echo "Merge conflicts:" >&2
    echo "$conflicts" >&2
    echo "Resolve them, keep the fork's own changes, run 'git commit', then run 'make upstream-sync' again." >&2
    exit 1
  fi
  git commit -q --no-edit
fi

echo "==> Installing dependencies..."
pnpm install --frozen-lockfile
echo "==> TypeScript typecheck..."
pnpm typecheck
echo "==> TypeScript unit tests..."
pnpm test
echo "==> Go build..."
(cd server && go build ./...)

echo "✓ main contains $UPSTREAM_REPO $tag and passes its checks"
