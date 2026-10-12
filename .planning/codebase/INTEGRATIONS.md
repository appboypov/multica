---
last_mapped_commit: 80fb7a97dc7c0b6e79ccfd92174fd7e124541d09
last_mapped_at: 2026-10-12
---
# External Integrations

**Analysis Date:** 2026-10-12

All integrations are opt-in and gated by environment variables read at server boot (`server/cmd/server/main.go`, `server/cmd/server/router.go`). An unset gate disables the feature and logs a `... integration disabled (... not set)` line instead of failing. Operator reference: `apps/docs/content/docs/environment-variables.mdx`, `SELF_HOSTING_ADVANCED.md`, `.env.example` (template present; never copy real values from `.env`, `.env.make`, `.env.worktree`).

## APIs & External Services

**Agent runtimes (local CLIs driven by the daemon):**
- The `multica` CLI (`server/cmd/multica/`) runs a local agent daemon (`server/internal/daemon/daemon.go`) that claims tasks from the server and executes AI coding CLIs as subprocesses under each tool's own credentials
  - SDK/Client: per-runtime backends in `server/pkg/agent/` (`claude.go`, `codex.go`, `cursor.go`, `copilot.go`, `opencode.go`, `opencode_v2.go`, `openclaw.go`, `hermes.go`, `kimi.go`, `kiro.go`, `qwen.go`, `qwenpaw.go`, `qoder.go`, `pi.go`, `grok.go`, `codebuddy.go`, `codearts.go`, `deveco.go`, `dim.go`, `dsh.go`, `mcode.go`, `reasonix.go`, `traecli.go`, `zeroclaw.go`, `antigravity.go`, ACP protocol in `acp_*.go`)
  - Default command names: `scripts/agent-cli-command-names.txt` (`agy`, `claude`, `codebuddy`, `codearts`, `codex`, `copilot`, `cursor-agent`, `deveco`, `dim`, `dsh`, `grok`, `hermes`, `kimi`, `kiro-cli`, `mcode`, `omp`, `openclaw`, `opencode`, `pi`, `qodercli`, `qoderclicn`, `qwen`, `qwenpaw`, `reasonix`, `traecli`, `zeroclaw`)
  - Runtime identities that reuse a protocol family (e.g. `omp` → `pi` protocol) are declared once in `server/pkg/agent/builtin_runtimes.go`; add new forks there, and add new default commands to `scripts/agent-cli-command-names.txt`
  - Auth: each CLI's own login; daemon honours per-tool homes (`CODEX_HOME`, `HERMES_HOME`, `KIMI_CODE_HOME`, `QWEN_HOME`, `GROK_HOME`, `REASONIX_HOME`, `DSH_HOME`, `OPENCLAW_HOME`, `OPENCLAW_CONFIG_PATH`, `CODEBUDDY_CONFIG_DIR`, `QWENPAW_WORKING_DIR`, `MULTICA_OPENCLAW_PATH`, `MULTICA_CODEARTS_PATH`)
  - Task-scoped env injected into agents: `MULTICA_TOKEN`, `MULTICA_WORKSPACE_ID`, `MULTICA_AGENT_ID`, `MULTICA_AGENT_NAME`, `MULTICA_TASK_ID`, `MULTICA_DAEMON_PORT` (see `mergeEnv` in `server/pkg/agent/`)
  - MCP wiring for agents: `server/pkg/agent/mcp_config.go`, `server/pkg/agent/browser_mcp_config.go`, `server/internal/daemon/runtime_mcp.go`, remote MCP broker `server/internal/daemon/remote_mcp_broker.go` + `server/pkg/remotemcp/`
  - Real-agent smoke tests are gated: build tag `agentintegration` + `MULTICA_RUN_REAL_AGENT_SMOKE=1` (`AGENTS.md`)

**LLM assist layer (server-side, OpenAI-compatible):**
- Chat auto-titles (`server/internal/handler/chat_title.go`) and chat follow-up quick actions (`server/internal/service/chat_quick_actions_generate.go`)
  - SDK/Client: `github.com/openai/openai-go/v3` wrapped by `server/pkg/llm/client.go` (the only allowed importer)
  - Auth: `MULTICA_LLM_API_KEY`; endpoint `MULTICA_LLM_BASE_URL`; `MULTICA_LLM_DEFAULT_MODEL`, `MULTICA_LLM_MAX_RETRIES` (0–5, invalid stops boot), `MULTICA_LLM_DISABLE_THINKING`
  - Both key and base URL empty → disabled client, zero upstream requests

**Chat channels (bot integrations, `server/internal/integrations/`):**
- Shared channel engine: `server/internal/integrations/channel/`; per-installation secrets are encrypted at rest with a per-integration secretbox key loaded via `secretbox.LoadKey` in `server/cmd/server/router.go`
- Lark / Feishu - IM bot via Open Platform HTTP API (`https://open.feishu.cn`, `https://open.larksuite.com`), device-flow registration at `accounts.feishu.cn` / `accounts.larksuite.com` (`server/internal/integrations/lark/`)
  - Auth: `MULTICA_LARK_SECRET_KEY` (gate); overrides `MULTICA_LARK_HTTP_BASE_URL`, `MULTICA_LARK_CALLBACK_BASE_URL`, `MULTICA_LARK_REGISTRATION_DOMAIN`, `MULTICA_LARK_REGISTRATION_LARK_DOMAIN`, `MULTICA_LARK_WS_PROXY_URL`
- Slack - BYO per-installation Socket Mode (`server/internal/integrations/slack/`)
  - SDK/Client: `github.com/slack-go/slack` v0.29.0
  - Auth: `MULTICA_SLACK_SECRET_KEY` (gate); bot/app tokens supplied per installation via `POST /api/workspaces/{id}/slack/install/byo`
- DingTalk - BYO per-installation Stream connection, REST at `https://api.dingtalk.com` (`server/internal/integrations/dingtalk/client.go`)
  - Auth: `MULTICA_DINGTALK_SECRET_KEY` (gate)
- WeCom (企业微信 smart bot) - long-lived WebSocket to `wss://openws.work.weixin.qq.com` (`server/internal/integrations/wecom/`)
  - Auth: `MULTICA_WECOM_SECRET_KEY` (gate); `MULTICA_WECOM_DEFAULT_LOCALE`, `MULTICA_WECOM_MEDIA_ALLOW_CIDRS` (SSRF allow-list), `MULTICA_WECOM_TRACE`
  - Multi-replica outbound needs the Redis relay (see Caching)
- Telegram - BYO bot token, long polling against `https://api.telegram.org` (`server/internal/integrations/telegram/api.go`)
  - Auth: `MULTICA_TELEGRAM_SECRET_KEY` (gate)
- Frontend surfaces: `packages/views/{lark,slack,dingtalk,wecom,telegram}/`, query hooks in `packages/core/{lark,slack,dingtalk,wecom,telegram}/`

**Composio (third-party app connections / MCP apps):**
- `server/internal/integrations/composio/` service over SDK `server/pkg/composio/` (REST root `https://backend.composio.dev/api/v3.1`, MCP sessions on `mcp.composio.dev`)
  - Auth: `COMPOSIO_API_KEY` (gate, plus feature flag via `featureflags.ComposioMCPAppsEnabled`), `COMPOSIO_STATE_SECRET` (HMAC-signed OAuth state), `COMPOSIO_CALLBACK_BASE_URL`, `COMPOSIO_WEBHOOK_SECRET` (`server/pkg/composio/webhook.go`)
  - Frontend: `packages/core/composio/`

**Version control providers:**
- GitHub App - PR mirroring onto issues, CI/mergeability snapshots (`server/internal/handler/github.go`, `server/internal/integrations/ghsnapshot/client.go` → `https://api.github.com`)
  - Auth: `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY` (PEM, double-quoted in env), `GITHUB_APP_SLUG`, `GITHUB_WEBHOOK_SECRET` (HMAC-SHA256); unset → nil client, enrichment silently off
- Forgejo / Gitea / GitLab - token-based per-workspace connections (`server/internal/integrations/vcs/forgejo.go`, `server/internal/integrations/vcs/gitlab.go`, `server/internal/integrations/vcs/vcs.go`)
  - Auth: `MULTICA_VCS_SECRET_KEY` encrypts stored access tokens; `MULTICA_VCS_INTEGRATION_ENABLED` (compose defaults `true`; `make selfhost*` generates the key)
- GitHub raw/API fetch for skill imports (`server/internal/handler/skill.go`)
  - Auth: optional `GITHUB_TOKEN` (sent only to GitHub hosts)
- Frontend: `packages/core/github/`, `packages/core/vcs/`

**Plugins:**
- Installable plugins with sandboxed HTML surfaces and MCP hook contributions (`server/pkg/plugincontract/`, plugin routes under `/api/workspaces/{id}/plugins` in `server/cmd/server/router.go`, `packages/plugin-sdk/`, examples in `examples/plugins/`)
  - Config: `MULTICA_PLUGIN_SECRET_KEY` (encrypts plugin secrets), `MULTICA_PLUGIN_SURFACE_ORIGIN` (separate cookie-free origin), `MULTICA_PLUGIN_API_URL`, `MULTICA_PLUGIN_DIR`, `MULTICA_PLUGIN_DEV_ORIGINS`, `MULTICA_PLUGIN_DEV_CA`

**Multica Cloud (upstream managed service, off for self-host):**
- Cloud runtime proxy, billing and entitlement policy (`server/internal/cloudruntime/client.go`, `server/internal/entitlement/doc.go`, `packages/core/billing/`, `packages/views/billing/`)
  - Config: `MULTICA_CLOUD_URL` (unset → disabled, `ErrDisabled`); web toggle `NEXT_PUBLIC_ENABLE_CLOUD_RUNTIME`
  - Cloud PATs (`mcn_` prefix) verified via `server/internal/auth/cloud_pat.go`

## Data Storage

**Databases:**
- PostgreSQL 17 (`pgvector/pgvector:pg17` image in `docker-compose.yml` and `docker-compose.selfhost.yml`)
  - Connection: `DATABASE_URL` (Makefile default `postgres://multica:multica@localhost:5432/multica?sslmode=disable`); optional read replica `DATABASE_REPLICA_URL` (+ `DATABASE_REPLICA_MAX_CONNS`, `DATABASE_REPLICA_MIN_CONNS`), `DATABASE_SEARCH_WORK_MEM_MB`, `PGCONNECT_TIMEOUT`, `MULTICA_DATABASE_STARTUP_TIMEOUT`, `MULTICA_DATABASE_CONNECT_TIMEOUT`
  - Client: `github.com/jackc/pgx/v5` `pgxpool`; typed queries generated by sqlc into `server/pkg/db/generated` from `server/pkg/db/queries/*.sql` (`server/sqlc.yaml`); regenerate with `make sqlc`
  - Migrations: `server/migrations/NNN_name.{up,down}.sql`, applied by the custom runner `server/cmd/migrate/main.go` (`schema_migrations` table, `pg_advisory_lock`, pre-migration hooks); containers run it from `docker/entrypoint.sh`
  - Extensions: `pgcrypto`, `pg_bigm`, `pg_trgm`, optional `pg_cron` (`server/migrations/001_init.up.sql`, `032_issue_search_index.up.sql`, `137_search_index_pg_trgm_extension.up.sql`, `076_task_usage_pgcron_extension.up.sql`)
  - Rules (`AGENTS.md`): no foreign keys; every index uses `CREATE [UNIQUE] INDEX CONCURRENTLY`, one statement per migration file; conditional DDL must be idempotent
  - Isolation: main checkout uses compose project `multica` (self-host stack, own volume `pgdata`); dev and worktrees share the `multica-dev` Postgres container with per-worktree databases (`scripts/ensure-postgres.sh`, `scripts/init-worktree-env.sh`, `scripts/drop-database.sh`)
- Test tooling also uses `pg` ^8.20.0 from Node (root `package.json` devDependencies) for E2E fixtures (`e2e/`)

**File Storage:**
- Local filesystem by default: `server/internal/storage/local.go` (`LOCAL_UPLOAD_DIR`, default `./data/uploads`; `LOCAL_UPLOAD_BASE_URL`); self-host mounts volume `backend_uploads` at `/app/data/uploads`
- S3 or S3-compatible (MinIO, R2, etc.) when `S3_BUCKET` is set: `server/internal/storage/s3.go`
  - Config: `S3_BUCKET`, `S3_REGION` (default `us-west-2`), `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_ENDPOINT_URL`, `S3_USE_PATH_STYLE`
  - Downloads: `ATTACHMENT_DOWNLOAD_MODE` (default `auto`), `ATTACHMENT_DOWNLOAD_URL_TTL` (default `30m`); presigned URLs via `storage.Presigner` (`server/internal/storage/storage.go`)
  - CDN: CloudFront signed URLs/cookies (`server/internal/auth/cloudfront.go`) - `CLOUDFRONT_DOMAIN`, `CLOUDFRONT_KEY_PAIR_ID`, `CLOUDFRONT_PRIVATE_KEY` or `CLOUDFRONT_PRIVATE_KEY_SECRET` (AWS Secrets Manager name), `COOKIE_DOMAIN`
- Desktop/mobile attachment pickers: `expo-image-picker`, `expo-document-picker` (`apps/mobile/package.json`)

**Caching:**
- Redis (optional) via `github.com/redis/go-redis/v9` (`server/internal/database/redis.go`)
  - Connection: `REDIS_URL` (`rediss://` enables TLS; comma-separated seeds), `REDIS_CLUSTER_MODE`, `REDIS_DISABLE_CLIENT_NAME`
  - Uses: realtime relay across replicas (`REALTIME_RELAY_MODE` = `sharded` default / `dual` / legacy; `REALTIME_RELAY_*` tuning in `server/cmd/server/main.go`), auth/contact-sales rate limiting (`RATE_LIMIT_AUTH`, `RATE_LIMIT_AUTH_VERIFY`, `RATE_LIMIT_CONTACT_SALES`, `RATE_LIMIT_TRUSTED_PROXIES` in `server/cmd/server/router.go`), channel WebSocket leases (`CHANNEL_WS_LEASE_BACKEND`, `CHANNEL_WS_LEASE_NAMESPACE`), WeCom claim/dedupe store, daemon token and empty-claim caches (`server/internal/auth/daemon_token_cache.go`, `server/internal/service/empty_claim_cache.go`)
  - Unset → single-node in-memory hub, rate limiting disabled (warns at boot). The local self-host stack in `docker-compose.selfhost.yml` defines no Redis service.
- In-process caches: membership/PAT caches (`server/internal/auth/membership_cache.go`, `server/internal/auth/pat_cache.go`)
- Client-side: TanStack Query cache (`packages/core/query-client.ts`)

## Authentication & Identity

**Auth Provider:**
- Custom (first-party) - implementation in `server/internal/auth/` and `server/internal/middleware/auth.go`
  - Email one-time code: `POST /auth/send-code`, `POST /auth/verify-code` (rate limited); codes sent via email service below; `MULTICA_DEV_VERIFICATION_CODE` fixed code for non-production only
  - Google OAuth: `POST /auth/google` exchanges the code at `https://oauth2.googleapis.com/token` and reads `https://www.googleapis.com/oauth2/v2/userinfo` (`server/internal/handler/auth.go`); config `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI` (default `$(FRONTEND_ORIGIN)/auth/callback`)
  - Sessions: JWT (`github.com/golang-jwt/jwt/v5`) signed with `JWT_SECRET` (`server/internal/auth/jwt.go`), cookie handling in `server/internal/auth/cookie.go` / `server/internal/auth/session.go`, `AUTH_TOKEN_TTL`, `COOKIE_DOMAIN`; production refuses empty/default secret (`jwtSecretBootError` in `server/cmd/server/main.go`)
  - Personal access tokens (`mul_` prefix) and Multica Cloud node PATs (`mcn_`) for CLI login (`server/cmd/multica/cmd_auth.go`)
  - Daemon tokens for `/api/daemon/*` (`middleware.DaemonAuth`, `server/internal/auth/daemon_token_cache.go`)
  - Signup policy: `ALLOW_SIGNUP`, `ALLOWED_EMAILS`, `ALLOWED_EMAIL_DOMAINS`, `DISABLE_WORKSPACE_CREATION`
  - Client token storage: web/desktop cookies + `packages/core/auth/`; desktop session coordination `apps/desktop/src/main/auth-session-coordinator.ts`; mobile `expo-secure-store`

## Monitoring & Observability

**Error Tracking:**
- PostHog exception autocapture in web/desktop (`packages/core/analytics/index.ts`, redaction in `packages/core/analytics/redact-exception.ts`, dedupe in `packages/core/analytics/exception-dedupe.ts`)

**Analytics:**
- PostHog server-side (`server/internal/analytics/client.go`, `server/internal/analytics/posthog.go`, event catalog `server/internal/analytics/events.go`)
  - Config: `POSTHOG_API_KEY` (empty → no-op), `POSTHOG_HOST` (default `https://us.i.posthog.com`), `ANALYTICS_ENVIRONMENT`, `ANALYTICS_DISABLED`
- PostHog client-side via `posthog-js`, initialised from `/api/config` (`posthog_key`, `posthog_host` in `packages/core/api/schemas.ts`; `packages/core/platform/auth-initializer.tsx`)
- Self-host telemetry: default ON, posts to the compiled-in collector `https://telemetry.multica.ai/v1/telemetry/events` (`server/internal/selfhosttelemetry/client.go`, `server/internal/selfhosttelemetry/config.go`); opt out with `DO_NOT_TRACK=1`

**Metrics:**
- Prometheus registry (`server/internal/metrics/`) on a separate listener when `METRICS_ADDR` is set (warns if not loopback)
- `GET /health/realtime` protected by `REALTIME_METRICS_TOKEN`; `GET /health`, `/readyz`, `/healthz` (`server/cmd/server/router.go`)
- pprof profiling server (`server/internal/profiling/server.go`); maintenance listener `MAINTENANCE_PORT` (`server/internal/maintenance/`)

**Logs:**
- `log/slog` with `github.com/lmittmann/tint` handler, `LOG_LEVEL` (`server/internal/logger/`); daemon log rotation via `lumberjack`
- Secret redaction helpers `server/pkg/redact/redact.go`
- Frontend logger `packages/core/logger.ts`; desktop dev log `apps/desktop/src/main/dev-log.ts`

## CI/CD & Deployment

**Hosting:**
- This fork: local self-hosted Docker Compose on the owner's Mac (`docker-compose.selfhost.yml` + `docker-compose.selfhost.build.yml`, started by `make selfhost-build`, waited on by `scripts/selfhost-wait.sh`); services bind to `127.0.0.1` only (backend `:8080`, web `:3000`) and need a reverse proxy for remote access
- Release: `make local-release` = `make build` (CLI/server/migrate into `server/bin/`) + `make selfhost-build` + `make desktop-app` (`Makefile`)
- Upstream sync: `make upstream-sync` → `scripts/upstream-sync.sh` (merges latest `multica-ai/multica` GitHub release tag via `gh release view`, strips `.github/workflows`, `.github/actions`, `.github/ci-paths.json`, `.github/RELEASING.md`, `.goreleaser.yml`, `.vercelignore`, `scripts/ci-scope.mjs`, then runs install, typecheck, unit tests, Go build)
- Upstream-only paths still in the tree: GHCR images in `docker-compose.selfhost.yml` (`make selfhost`), Helm chart `deploy/helm/multica/`, GitHub Releases publish config in `apps/desktop/electron-builder.yml`, installers `scripts/install.sh` / `scripts/install.ps1`

**Update paths (fork-redirected):**
- CLI `multica update`: always returns `cli.LocalReleaseUpdateMessage` (`server/cmd/multica/cmd_update.go`, `server/internal/cli/update.go`)
- Daemon self-update requests: answered with the same message (`server/internal/daemon/daemon.go` `runUpdate`)
- Desktop: `electron-updater` is configured but every automatic/manual check reports `LOCAL_RELEASE_UPDATE_MESSAGE` instead of hitting a feed (`apps/desktop/src/main/updater.ts`)
- Desktop CLI bootstrap still downloads from `https://github.com/multica-ai/multica/releases/latest/download` with `checksums.txt` verification when the bundled CLI is missing (`apps/desktop/src/main/cli-bootstrap.ts`); bundled CLI comes from `apps/desktop/scripts/bundle-cli.mjs`
- Mirror overrides honoured by CLI code: `MULTICA_RELEASE_API_BASE_URL`, `MULTICA_RELEASE_DOWNLOAD_BASE_URL` (`server/internal/cli/update.go`)

**CI Pipeline:**
- None. GitHub Actions disabled and workflow/GoReleaser/Vercel files removed (`AGENTS.md`, `scripts/upstream-sync.sh`). Verification runs locally with `make check` (`scripts/check.sh`)

## Environment Configuration

**Required env vars:**
- `DATABASE_URL`
- `JWT_SECRET` (hard requirement when `APP_ENV=production`)
- `FRONTEND_ORIGIN`, `MULTICA_APP_URL` (links in emails, redirects)
- `MULTICA_PUBLIC_URL` (absolute webhook URLs; never derived from Host headers), `MULTICA_DAEMON_SERVER_URL` (daemon setup URL when different)
- `CORS_ALLOWED_ORIGINS` / `ALLOWED_ORIGINS`, `MULTICA_TRUSTED_PROXIES`
- One of `RESEND_API_KEY` or `SMTP_HOST` for real login emails (otherwise codes print to the server log)
- Web container: `REMOTE_API_URL` (default `http://backend:8080` in `Dockerfile.web`), `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_WS_URL`, `DOCS_URL`
- Mobile: `EXPO_PUBLIC_API_URL`, `EXPO_PUBLIC_WEB_URL`

**Secrets location:**
- Root `.env` (main checkout) / `.env.worktree` (worktrees), generated from `.env.example`; `make selfhost*` writes random `JWT_SECRET`, `POSTGRES_PASSWORD`, `MULTICA_VCS_SECRET_KEY` (`Makefile`). Files exist; contents must never be read or committed.
- Per-integration secretbox keys (`MULTICA_{LARK,SLACK,DINGTALK,WECOM,TELEGRAM,VCS,PLUGIN}_SECRET_KEY`) encrypt per-installation credentials stored in Postgres
- Optional AWS Secrets Manager for the CloudFront private key (`CLOUDFRONT_PRIVATE_KEY_SECRET`, `server/internal/auth/cloudfront.go`)
- Mobile variant env files `apps/mobile/.env.staging`, `apps/mobile/.env.production` (present, not read); Apple signing via `EXPO_APPLE_TEAM_ID`; notarization env `APPLE_ID`, `APPLE_APP_SPECIFIC_PASSWORD`, `APPLE_TEAM_ID` (`apps/desktop/electron-builder.yml`, unused by the fork's unsigned build)

## Email

- Delivery priority SMTP relay → Resend API → stdout (`server/internal/service/email.go`)
  - SMTP: `SMTP_HOST`, `SMTP_PORT` (default 25), `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, `SMTP_TLS`, `SMTP_TLS_INSECURE`, `SMTP_EHLO_NAME`; STARTTLS when advertised, LOGIN-auth fallback
  - Resend: `github.com/resend/resend-go/v2`, `RESEND_API_KEY`, `RESEND_FROM_EMAIL` (compose default `noreply@multica.ai`)
  - Messages: login verification codes, workspace invitations (subject fields sanitised and capped at 60 runes)

## Realtime Transport

- Browser/desktop/mobile ↔ server: WebSocket `GET /ws` (`server/internal/realtime/hub.go`), client `packages/core/api/ws-client.ts`, sync hooks `packages/core/realtime/use-realtime-sync.ts`, mobile `apps/mobile/lib/use-ws-subscriptions.ts`; events patch/invalidate TanStack Query caches
- Daemon ↔ server: REST under `/api/daemon/*` (`/register`, `/heartbeat`, `/runtimes/{runtimeId}/tasks/claim`, task session/plugin-hook endpoints) plus WebSocket `/api/daemon/ws` (`server/internal/daemonws/`, `server/internal/daemon/wsrpc.go`); wire types in `server/pkg/protocol/`
- Desktop manages the local daemon process (`apps/desktop/src/main/daemon-manager.ts`, `apps/desktop/src/main/daemon-profile.ts`)
- Web proxies `/api`, `/v1`, `/ws`, `/auth`, `/uploads`, `/health` to `REMOTE_API_URL` (`apps/web/next.config.ts`, runtime rewrites in `apps/web/proxy.ts`)

## Webhooks & Callbacks

**Incoming (public, outside session auth — `server/cmd/server/router.go`):**
- `POST /api/webhooks/autopilots/{token}` - autopilot triggers; URL token is the credential
- `POST /api/webhooks/github` - GitHub App events, HMAC-SHA256 via `GITHUB_WEBHOOK_SECRET`; `GET /api/github/setup` post-install callback
- `POST /api/webhooks/vcs/{connectionId}` - Forgejo/Gitea/GitLab, per-connection signature
- `POST /api/webhooks/stripe` - raw body + `Stripe-Signature` forwarded to Multica Cloud for verification (`HandleCloudBillingStripeWebhook`)
- `GET /api/integrations/composio/callback` - Composio OAuth completion, identity from HMAC-signed `state`
- Binding-token redemption for chat bots: `POST /api/{lark,slack,dingtalk,wecom,telegram}/binding/redeem` (session-authenticated)
- Chat channels use outbound connections (Slack Socket Mode, DingTalk Stream, WeCom WebSocket, Telegram long polling), so they need no public inbound URL

**Outgoing:**
- Queued webhook deliveries with Postgres-backed queue/lease, 5 attempts, concurrency 4 (`server/internal/handler/webhook_delivery_worker.go`)
- Chat-channel replies to Lark, Slack, DingTalk, WeCom, Telegram APIs (see Chat channels)
- Self-host telemetry to `https://telemetry.multica.ai` unless `DO_NOT_TRACK` (see Analytics)
- PostHog event capture (server and client) when keys are configured
- Multica Cloud requests when `MULTICA_CLOUD_URL` is set

---

*Integration audit: 2026-10-12*
