#!/usr/bin/env bash
# Merges the latest multica-ai/multica release into this fork's main, keeps the
# fork free of the files it removed, then installs and builds the Go server.
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
  apps/docs
  apps/ui-lab
  apps/mobile
  CONTRIBUTING.md
  # Upstream install, self-hosting and project docs.
  CLI_INSTALL.md README.zh.md SELF_HOSTING.md SELF_HOSTING_ADVANCED.md SELF_HOSTING_AI.md VISION.md VISION.zh.md
  deploy
  scripts/install.{sh,ps1,test.sh,ps1.test.ps1}
  scripts/helm-config.test.sh
  packages/views/common/cli-install-command{,.test}.tsx
  # Marketing site and its MDX pipeline.
  "apps/web/app/(landing)"
  apps/web/app/{custom.css,sitemap.ts}
  apps/web/{content,features/landing,public/images,public/usecases,source.config.ts}
  apps/web/lib/{docs-href,public-host,use-case-locale-fallback}{,.test}.ts
  apps/web/lib/{use-cases-i18n,use-cases-source}.ts
  server/internal/handler/contact_sales{,_test}.go
  server/pkg/db/queries/contact_sales.sql
  server/pkg/db/generated/contact_sales.sql.go
  # Chat adapters: Lark/Feishu, DingTalk, Telegram, WeCom and Slack.
  apps/web/app/{dingtalk,lark,slack,telegram,wecom}
  packages/core/{dingtalk,lark,slack,telegram,wecom}
  packages/core/types/{dingtalk,lark,slack,telegram,wecom}.ts
  packages/views/{dingtalk,lark,slack,telegram,wecom}
  packages/views/settings/components/{dingtalk,lark,slack,telegram,wecom}-{mark.tsx,tab.tsx,tab.test.tsx}
  packages/views/settings/components/{channels-tab.tsx,channels-tab.test.tsx,integration-channel-icon.tsx}
  packages/views/agents/components/tabs/integrations-tab{,.test}.tsx
  packages/views/editor/utils/preprocess-channel-media.test.ts
  server/internal/integrations/{channel,dingtalk,lark,slack,telegram,wecom}
  server/internal/channelmedia
  server/internal/handler/{dingtalk,lark,slack,telegram,wecom_web}{,_test}.go
  server/internal/handler/{wecom_install_error_matrix,channel_context_lock_order,channel_new_e2e,channel_title_regression}_test.go
  server/internal/handler/{chat_archive_cancel,chat_channel_command_visibility,chat_channel_debounce_boundary,daemon_claim_channel_type,issue_channel_media_merge}_test.go
  server/cmd/server/{channel_lease_config,dingtalk_reactions_integration}_test.go
  server/cmd/server/wecom_{bubble,claim_redis,dedup,replier}_wiring_test.go
  server/internal/daemon/execenv/channel_type{,_test}.go
  server/internal/daemon/execenv/runtime_config_channel_context_test.go
  server/internal/metrics/{channel_lease,channel_media,wecom,wecom_test}.go
  server/internal/service/channel_media_reconciler{,_test}.go
  server/internal/service/{issue_channel_media,task_channel_command_media,task_channel_media_race,task_channel_seal_race}_test.go
  server/pkg/db/queries/dingtalk.sql
  server/pkg/db/generated/dingtalk.sql.go
  # Desktop updater, upstream CLI download and non-Mac packaging.
  apps/desktop/build/{icon.ico,icon.png,icons}
  apps/desktop/src/main/{updater,updater-preferences,cli-release-asset}{,.test}.ts
  apps/desktop/src/main/cli-bootstrap.ts
  apps/desktop/src/shared/updater-types.ts
  apps/desktop/src/renderer/src/components/{update-notification,updates-settings-tab}{,.test}.tsx
  # Perf harness, image budget, upstream skills and GitHub issue templates.
  e2e/perf playwright.perf.config.ts
  scripts/{perf-compare.mjs,perf-compare.test.sh,check-image-budget.mjs,check-image-budget.test.mjs}
  .agents skills-lock.json .github/ISSUE_TEMPLATE docs/assets/pr-5537
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
  git rm -r -q -f --ignore-unmatch -- "${FORK_REMOVED_PATHS[@]}"
  # The upstream lockfile still lists the removed apps; pnpm rewrites it for
  # the fork's workspace and resolves conflict markers in it.
  pnpm install --lockfile-only
  git add pnpm-lock.yaml
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
echo "==> Go build..."
(cd server && go build ./...)

echo "✓ main contains $UPSTREAM_REPO $tag and builds"
