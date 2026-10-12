---
last_mapped_commit: 80fb7a97dc7c0b6e79ccfd92174fd7e124541d09
last_mapped_at: 2026-10-12
---
# Technology Stack

**Analysis Date:** 2026-10-12

## Languages

**Primary:**
- Go 1.26.6 (`go` directive in `server/go.mod`) - backend API server, `multica` CLI + agent daemon, migration runner, maintenance and backfill jobs (`server/cmd/*`, `server/internal/*`, `server/pkg/*`)
- TypeScript ^5.9.3 (`catalog:` entry in `pnpm-workspace.yaml`) - every frontend workspace: `apps/web/`, `apps/desktop/`, `apps/docs/`, `apps/ui-lab/`, `packages/core/`, `packages/ui/`, `packages/views/`, `packages/plugin-sdk/`; mobile pins its own `~5.9.0` in `apps/mobile/package.json`

**Secondary:**
- SQL (PostgreSQL dialect) - 563 numbered migrations (1178 `.up.sql`/`.down.sql` files) in `server/migrations/`, 65 sqlc query files in `server/pkg/db/queries/`
- Bash / POSIX sh - repo automation in `scripts/*.sh` (`scripts/dev-env.sh`, `scripts/check.sh`, `scripts/upstream-sync.sh`, `scripts/ensure-postgres.sh`), container entrypoint `docker/entrypoint.sh`
- JavaScript (ESM `.mjs`) - build/check helpers in `scripts/*.mjs` and `apps/desktop/scripts/*.mjs`
- MDX - documentation content in `apps/docs/content/docs/` and `apps/web/content/` (compiled by fumadocs-mdx)
- PowerShell - Windows CLI installer `scripts/install.ps1`
- YAML - Helm chart `deploy/helm/multica/`, Compose files, `apps/desktop/electron-builder.yml`

## Runtime

**Environment:**
- Node.js >=22 (`engines` in `package.json`; `.nvmrc` pins `22`; images use `node:22-alpine` in `Dockerfile.web`)
- Go toolchain 1.26 (`golang:1.26-alpine` builder in `Dockerfile`); runtime image `alpine:3.21` with `ca-certificates` + `tzdata`
- Electron ^39.2.6 for the desktop app (`apps/desktop/package.json`)
- React Native 0.83.6 on Expo SDK ~55 for iOS (`apps/mobile/package.json`, `apps/mobile/app.config.ts`)
- PostgreSQL 17 via `pgvector/pgvector:pg17` image (`docker-compose.yml`, `docker-compose.selfhost.yml`)

**Package Manager:**
- pnpm 10.28.2 (`packageManager` in `package.json`; activated through corepack in `Dockerfile.web`)
- Workspaces: `apps/*` and `packages/*` (`pnpm-workspace.yaml`)
- Shared versions live in the pnpm `catalog:` block of `pnpm-workspace.yaml`; workspaces reference them as `"catalog:"`. Mobile deliberately pins Expo/React Native-compatible versions in its own manifest (React 19.2.0, Tailwind 3, tailwind-merge 2) instead of the catalog.
- `pnpm.overrides` in `package.json` pins security-fixed transitive versions (`postcss 8.5.26`, `uuid 11.1.1`, `shell-quote 1.9.0`, `brace-expansion` per minimatch major, `builder-util-runtime 9.7.0`, `@xmldom/xmldom ^0.8.13`)
- `pnpm.onlyBuiltDependencies`: `esbuild`, `electron`
- Lockfile: present (`pnpm-lock.yaml`, `server/go.sum`)
- Go modules: `server/go.mod` (module path `github.com/multica-ai/multica/server`); `tool golang.org/x/vuln/cmd/govulncheck` is declared as a Go tool dependency

## Frameworks

**Core (backend, `server/go.mod`):**
- `github.com/go-chi/chi/v5` v5.3.2 - HTTP router (`server/cmd/server/router.go`)
- `github.com/go-chi/cors` v1.2.2 - CORS middleware
- `github.com/gorilla/websocket` v1.5.3 - realtime hub `/ws` (`server/internal/realtime/hub.go`) and daemon socket `/api/daemon/ws` (`server/internal/daemonws/`)
- `github.com/jackc/pgx/v5` v5.11.0 - Postgres driver + `pgxpool` (primary + optional read replica)
- sqlc v1.31.1 (invoked via `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1` in `Makefile` target `sqlc`) - generates `server/pkg/db/generated` from `server/sqlc.yaml`
- `github.com/spf13/cobra` v1.10.2 + `pflag` - `multica` CLI (`server/cmd/multica/`)
- `github.com/robfig/cron/v3` v3.0.1 - cron expressions for autopilot schedules / DB-backed scheduler (`server/internal/scheduler/`)

**Core (frontend):**
- Next.js ^16.3.4 (webpack mode, `--webpack` flag) - web app `apps/web/` (`apps/web/next.config.ts`)
- Next.js ^15.5.16 + Fumadocs (`fumadocs-core`/`fumadocs-ui` ^15.5.2, `fumadocs-mdx` ^12.0.3) - docs site `apps/docs/`
- Electron ^39.2.6 + electron-vite ^5.0.0 + electron-builder ^26.0.12 + react-router-dom ^7.6.0 - desktop `apps/desktop/`
- Expo ~55.0.23 + expo-router ~55.0.14 + NativeWind ^4.1.23 - mobile `apps/mobile/`
- Vite ^8.0.1 - design playground `apps/ui-lab/`
- React 19.2.3 / react-dom 19.2.3 (catalog) - all web/desktop surfaces
- TanStack Query ^5.96.2 (server state), Zustand ^5.0.0 (client state), zod ^4.1.5 (API response validation via `parseWithFallback` in `packages/core/api/schema.ts`)
- Tailwind CSS ^4 (`@tailwindcss/postcss`, `@tailwindcss/vite`) + shadcn ^4.1.0 + Base UI `@base-ui/react` ^1.3.0 - styling and primitives (`packages/ui/`)
- Tiptap 3.31.3 (pinned exactly, ~20 `@tiptap/*` packages) - rich-text editor (`packages/views/editor/`)
- i18next ^26.0.8 + react-i18next ^17.0.6 - localization (`packages/views/locales/`, `apps/mobile/lib/i18n/`)

**Testing:**
- Go `testing` with race detector (`scripts/test-go.sh --race`, `Makefile` target `test`); `github.com/go-redis/redismock/v9` v9.2.0 for Redis fakes; DB fixtures in `server/internal/testutil/`
- Vitest ^4.1.0 + jsdom ^29.0.1 + Testing Library (`@testing-library/react` ^16.3.2, `jest-dom` ^6.9.1, `user-event` ^14.6.1) - every TS workspace (`apps/web/vitest.config.ts`, `apps/desktop/vitest.config.ts`, `apps/mobile/vitest.config.ts`)
- Playwright ^1.58.2 - E2E specs in `e2e/` (`playwright.config.ts`, `playwright.iframe.config.ts`, `playwright.perf.config.ts`)
- Shell test scripts `scripts/*.test.sh`, `apps/mobile/scripts/ios-run.test.sh`

**Build/Dev:**
- Turborepo ^2.5.4 - task graph (`turbo.json`: `build`, `typecheck`, `test`, `lint`, `mdx`, `dev`, hash-only `cache-inputs`)
- Make - canonical entry point (`Makefile`; see Configuration)
- ESLint ^9 with shared configs `packages/eslint-config/{base,react,next}.js`; `eslint-plugin-i18next` ^6.1.4; mobile uses `eslint-config-expo` ~55
- Shared tsconfigs `packages/tsconfig/{base,react-library}.json`
- knip ^6.32.2 - unused export/dependency checks (`knip.jsonc`)
- Custom guards: `scripts/check-ui-wildcard-exports.mjs`, `scripts/check-ui-radius-tokens.mjs`, `scripts/check-image-budget.mjs`, `scripts/generate-reserved-slugs.mjs`
- Docker multi-stage builds: `Dockerfile` (Go binaries), `Dockerfile.web` (Next.js standalone)

## Key Dependencies

**Critical (backend, `server/go.mod`):**
- `github.com/jackc/pgx/v5` v5.11.0 - only database driver; sqlc emits `pgx/v5` code (`server/sqlc.yaml`)
- `github.com/redis/go-redis/v9` v9.22.0 - optional realtime relay, rate limiting, channel WebSocket leases, caches (`server/internal/database/redis.go`)
- `github.com/golang-jwt/jwt/v5` v5.3.1 - session JWTs (`server/internal/auth/jwt.go`)
- `github.com/openai/openai-go/v3` v3.61.0 - server-side LLM assist layer, imported ONLY by `server/pkg/llm/client.go` (enforced by `TestOpenAISDKIsImportedOnlyByThisPackage`)
- `github.com/aws/aws-sdk-go-v2` v1.47.0 + `service/s3` v1.113.0 + `service/secretsmanager` v1.49.0 - attachment storage (`server/internal/storage/s3.go`) and CloudFront key loading (`server/internal/auth/cloudfront.go`)
- `github.com/resend/resend-go/v2` v2.28.0 - transactional email (`server/internal/service/email.go`)
- `github.com/slack-go/slack` v0.29.0 - Slack Socket Mode integration (`server/internal/integrations/slack/`)
- `github.com/prometheus/client_golang` v1.24.1 - metrics (`server/internal/metrics/`)
- `github.com/yuin/goldmark` v1.8.6, `github.com/tdewolff/parse/v2` v2.8.16 - Markdown/HTML processing for channel output
- `github.com/oklog/ulid/v2`, `github.com/google/uuid` - identifiers (`server/pkg/dbid/`)
- `github.com/lmittmann/tint` v1.2.0 + `log/slog`, `gopkg.in/natefinch/lumberjack.v2` v2.2.1 - structured/rotated logs (`server/internal/logger/`)
- `github.com/go-resty/resty/v2` v2.17.2 - outbound HTTP client
- `github.com/pelletier/go-toml/v2`, `gopkg.in/yaml.v3` - agent CLI config files and feature-flag YAML (`server/pkg/featureflag/`)
- `github.com/mattn/go-shellwords` - parsing agent launch commands (`server/pkg/agent/`)

**Critical (frontend):**
- `@tanstack/react-query` (catalog) - all server state, shared by web/desktop/mobile through `packages/core/`
- `zustand` (catalog) - client state stores in `packages/core/` (shared) and app-local stores
- `zod` (catalog) - response schemas in `packages/core/api/schemas.ts`
- `posthog-js` ^1.176.1 - product analytics + exception capture (`packages/core/analytics/index.ts`)
- `electron-updater` ^6.8.3 - desktop update plumbing, short-circuited by the fork (`apps/desktop/src/main/updater.ts`)
- `mermaid` ^11.14.0, `katex` ^0.16.45, `shiki` ^3.21.0 / `@shikijs/*` ^4 (mobile), `react-markdown` ^10.1.0, `remark-*`/`rehype-*` - Markdown rendering in `packages/ui/markdown/`, `packages/views/`, `apps/mobile/lib/markdown/`
- `recharts` 3.8.0 - dashboards (`packages/ui/`, `packages/views/dashboard/`)
- `@dnd-kit/*` - drag and drop (board views, desktop tabs)
- `expo-secure-store` ~55 - mobile token storage

**Infrastructure:**
- Compose: `docker-compose.yml` (dev Postgres only, project `multica-dev`, `127.0.0.1:5432`), `docker-compose.selfhost.yml` (project `multica`: `postgres`, `backend`, `frontend`), `docker-compose.selfhost.build.yml` (builds `multica-backend:dev` / `multica-web:dev` from the checkout)
- Helm chart `deploy/helm/multica/` (Chart `0.1.0`, `appVersion: latest`) - upstream Kubernetes path; not used by this fork's local release
- Postgres extensions created by migrations: `pgcrypto` (`server/migrations/001_init.up.sql`), `pg_bigm` (`server/migrations/032_issue_search_index.up.sql`), `pg_cron` (`server/migrations/076_task_usage_pgcron_extension.up.sql`, wrapped in DO/EXCEPTION so it is optional), `pg_trgm` (`server/migrations/137_search_index_pg_trgm_extension.up.sql`)

## Configuration

**Environment:**
- Single root env file drives every component: `.env` (main checkout) or `.env.worktree` (linked worktrees), selected in `Makefile` (`MAIN_ENV_FILE`, `WORKTREE_ENV_FILE`, `ENV_FILE`). `.env.example` is the documented template; `.env.make` is a generated make-safe copy (multi-line values stripped) - all present, contents not read.
- `Makefile` derives and exports defaults: `POSTGRES_DB/USER/PASSWORD=multica`, `POSTGRES_PORT=5432`, `PORT=8080`, `FRONTEND_PORT=3000`, `DATABASE_URL`, `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_WS_URL`, `GOOGLE_REDIRECT_URI`, `MULTICA_SERVER_URL`, `LOCAL_UPLOAD_BASE_URL`, `MULTICA_PUBLIC_URL`, `MULTICA_APP_URL`
- Server reads configuration exclusively through `os.Getenv` at boot (`server/cmd/server/main.go`, `server/cmd/server/router.go`, `server/internal/handler/config.go`); invalid critical knobs (`JWT_SECRET` in production, `MULTICA_LLM_MAX_RETRIES`, `MULTICA_LLM_DISABLE_THINKING`, `REALTIME_RELAY_MODE`) stop the boot
- Key configs required: `DATABASE_URL`, `JWT_SECRET` (mandatory when `APP_ENV=production`), `FRONTEND_ORIGIN`, `MULTICA_APP_URL`; everything else is opt-in (see `.planning/codebase/INTEGRATIONS.md`)
- Feature flags: `MULTICA_FEATURE_FLAGS_FILE` (YAML) layered with `FF_<KEY>` env overrides (`server/pkg/featureflag/doc.go`)
- Web runtime: `REMOTE_API_URL`, `DOCS_URL`, `CORS_ALLOWED_ORIGINS`, `STANDALONE`, `NEXT_PUBLIC_APP_VERSION`, `NEXT_PUBLIC_ENABLE_CLOUD_RUNTIME` (`apps/web/next.config.ts`, `apps/web/config/runtime-urls`, `apps/web/proxy.ts`)
- Mobile: `APP_ENV`, `EXPO_PUBLIC_API_URL`, `EXPO_PUBLIC_WEB_URL`, `EXPO_APPLE_TEAM_ID`, `EXPO_BUNDLE_IDENTIFIER_{DEV,PROD}` (`apps/mobile/app.config.ts`); per-variant files `apps/mobile/.env.staging`, `apps/mobile/.env.production` exist (not read)
- Turbo `globalEnv` invalidates caches on `DATABASE_URL`, `PORT`, `FRONTEND_PORT`, `FRONTEND_ORIGIN`, `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_WS_URL`, `MULTICA_SERVER_URL`, `DOCS_URL`, `COMPOSE_PROJECT_NAME`, `POSTGRES_DB`, `POSTGRES_PORT`, `DESKTOP_RENDERER_PORT`, `DESKTOP_APP_SUFFIX` (`turbo.json`)
- Operator docs for every variable: `apps/docs/content/docs/environment-variables.mdx` (+ `zh`, `ja`, `ko`, `fr`), `SELF_HOSTING.md`, `SELF_HOSTING_ADVANCED.md`, `SELF_HOSTING_AI.md`

**Build:**
- `Makefile` targets (run `make help`):
  - Environments: `up`/`down`/`status`/`list`/`destroy`/`gc`/`env-exec` (wrap `scripts/dev-env.sh`, `C=` picks components `api,web,daemon,desktop`)
  - One-click: `setup`, `start`, `stop`, `check` (`scripts/check.sh`: typecheck, TS tests, Go tests, Playwright E2E), `dev` (`scripts/dev.sh`)
  - Worktrees: `worktree-env`, `setup-main`/`start-main`/`stop-main`/`check-main`, `setup-worktree`/`start-worktree`/`stop-worktree`/`check-worktree`, `remove-worktree`
  - Individual: `server`, `daemon`, `cli`, `multica`, `build` (writes `server/bin/{server,multica,migrate}` with `-ldflags -X main.version/commit/date`), `test`, `api-dev`, `web-dev`, `desktop-dev`
  - Database: `db-up`, `db-down`, `db-drop`, `db-reset`, `migrate-up`, `migrate-down`, `sqlc`
  - Self-hosting: `selfhost` (pull GHCR images), `selfhost-build` (build from checkout), `selfhost-stop`; both generate random `JWT_SECRET`, `POSTGRES_PASSWORD`, `MULTICA_VCS_SECRET_KEY` into a fresh `.env`
  - Fork: `upstream-sync` (`scripts/upstream-sync.sh`), `local-release` (= `build` + `selfhost-build` + `desktop-app`), `desktop-app` (unsigned macOS package via `pnpm -C apps/desktop package -- --mac --publish never`, installed to `/Applications/Multica.app`)
  - `clean`
- Root `package.json` scripts: `build`/`typecheck`/`test`/`lint` run through turbo and exclude `@multica/mobile`; `dev:web`, `dev:desktop`, `dev:docs`, `dev:mobile*`, `ios:mobile*`, `knip`, `check:ui-exports`, `check:ui-radii`, `ui:add`, `generate:reserved-slugs`, `dev:ui-lab`
- `server/sqlc.yaml`: engine `postgresql`, queries `pkg/db/queries/`, schema `migrations/`, output package `db` in `pkg/db/generated`, `emit_json_tags`, `emit_empty_slices`, override `issue_wakeup.condition` → `encoding/json.RawMessage`
- `apps/web/next.config.ts`: `output: "standalone"` when `STANDALONE=true`, `transpilePackages: ["@multica/core","@multica/ui","@multica/views"]`, rewrites `/api`, `/v1`, `/ws`, `/health`, `/auth`, `/uploads` to `REMOTE_API_URL` and `/docs` to `DOCS_URL`
- `apps/desktop/electron-builder.yml`: appId `ai.multica.desktop`, `multica://` protocol, mac `dmg`+`zip` (notarize on; the fork's `desktop-app` target disables signing with `CSC_IDENTITY_AUTO_DISCOVERY=false`), linux `AppImage`/`deb`/`rpm`, win `nsis`; `apps/desktop/scripts/bundle-cli.mjs` bundles the Go CLI into the app
- `Dockerfile` builds `server`, `multica`, `migrate`, `maintenance`, `backfill_task_usage_hourly`, `backfill_codex_usage_cache` with `CGO_ENABLED=0`; `docker/entrypoint.sh` runs `./migrate up` then `exec ./server`
- `Dockerfile.web` builds `@multica/web` standalone and runs `node apps/web/server.js` as uid 1001 on port 3000

## Platform Requirements

**Development:**
- macOS (fork owner's machine; `desktop-app` target uses `ditto`/`osascript`), Linux and Windows supported upstream (`Makefile` handles `.exe` suffixes)
- Node 22 + pnpm 10.28.2 (corepack), Go 1.26, Docker with the Compose v2 plugin (`REQUIRE_COMPOSE` in `Makefile` rejects legacy `docker-compose` v1)
- `gh` CLI for `scripts/upstream-sync.sh` (reads latest `multica-ai/multica` release tag); git remotes `origin` = fork, `upstream` = `multica-ai/multica` (fetch-only)
- Xcode / iOS toolchain for `apps/mobile/scripts/ios-run.sh`
- Optional locally-installed agent CLIs for the daemon (`scripts/agent-cli-command-names.txt`)
- No remote CI: `.github/workflows`, `.github/actions`, `.goreleaser.yml`, `.vercelignore` are removed by `scripts/upstream-sync.sh` on every sync; only `.github/ISSUE_TEMPLATE/` and `.github/PULL_REQUEST_TEMPLATE.md` remain

**Production:**
- Fork release target is local only: `make local-release` produces the CLI in `server/bin/` (linked from `~/.local/bin/multica` per `AGENTS.md`), the self-hosted Docker Compose stack built from the checkout (`docker-compose.selfhost.yml` + `docker-compose.selfhost.build.yml`, project `multica`, backend `127.0.0.1:8080`, web `127.0.0.1:3000`, own Postgres volume `pgdata`, uploads volume `backend_uploads`), and the unsigned desktop app in `/Applications`
- Upstream targets (present in code, not used by the fork): GHCR images `ghcr.io/multica-ai/multica-backend` / `multica-web` (`docker-compose.selfhost.yml`), Helm chart `deploy/helm/multica/`, GitHub Releases for desktop/CLI artifacts

---

*Stack analysis: 2026-10-12*
