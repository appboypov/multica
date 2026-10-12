---
last_mapped_commit: 80fb7a97dc7c0b6e79ccfd92174fd7e124541d09
last_mapped_at: 2026-10-12
---
<!-- refreshed: 2026-10-12 -->

# Codebase Structure

**Analysis Date:** 2026-10-12

## Directory Layout

```
multica/
├── server/                     # Go backend, CLI, daemon (module github.com/multica-ai/multica/server, go 1.26)
│   ├── cmd/
│   │   ├── server/             # API server main, router, event listeners, sweepers, shutdown
│   │   ├── multica/            # Cobra CLI (`multica`), incl. `daemon` subcommand (cmd_*.go)
│   │   ├── migrate/            # Migration runner binary
│   │   ├── maintenance/        # Maintenance jobs binary
│   │   └── backfill_*/         # One-off data backfill binaries
│   ├── internal/
│   │   ├── handler/            # HTTP handlers, one file per resource (+ reserved_slugs.json)
│   │   ├── service/            # Domain services (issue, task, autopilot, plugin, wakeups, email)
│   │   ├── middleware/         # Auth, daemon/plugin auth, workspace gates, CSP, rate limits, logging
│   │   ├── realtime/           # Client WebSocket hub + Redis relays
│   │   ├── daemonws/           # Daemon WebSocket hub + notifier
│   │   ├── events/             # In-process event bus
│   │   ├── daemon/             # Local daemon runtime (execenv/, repocache/, processtree/)
│   │   ├── scheduler/          # DB-backed cron manager + jobs_*.go
│   │   ├── integrations/       # slack, lark, telegram, wecom, dingtalk, vcs, composio, channel, ghsnapshot
│   │   ├── issuestatus/ issueproperty/ issueposition/ issueguard/   # Issue domain helpers
│   │   ├── entitlement/ seatcapacity/ featureflags/ cloudruntime/   # Commercial/cloud gates
│   │   ├── auth/ storage/ skill/ analytics/ metrics/ logger/ util/  # Cross-cutting packages
│   │   ├── *backfill/          # Backfill implementations used by cmd/backfill_*
│   │   └── testutil/           # Go test DB/HTTP fixtures (db.go, http.go)
│   ├── pkg/
│   │   ├── db/queries/         # Hand-written SQL for sqlc (*.sql)
│   │   ├── db/generated/       # sqlc output (*.sql.go) — DO NOT EDIT
│   │   ├── agent/              # Agent CLI backends (claude.go, codex.go, cursor.go, …)
│   │   ├── protocol/           # WS event constants + messages
│   │   ├── eventcontract/      # Event contract definitions
│   │   ├── publicapi/v1/       # Public plugin API routes + openapi.yaml
│   │   ├── plugincontract/ remotemcp/ llm/ composio/ skillbundle/ featureflag/ redact/ dbid/ taskfailure/
│   ├── migrations/             # NNN_name.up.sql / NNN_name.down.sql (563+ pairs)
│   ├── sqlc.yaml               # sqlc config
│   └── bin/                    # Built binaries (gitignored)
├── packages/
│   ├── core/                   # @multica/core — headless logic (api/, platform/, realtime/, <feature>/)
│   ├── ui/                     # @multica/ui — primitives, tokens, markdown (components/ui, components/common, styles)
│   ├── views/                  # @multica/views — shared web/desktop pages (<feature>/, layout/, navigation/, locales/)
│   ├── plugin-sdk/             # @multica plugin SDK (index.ts, protocol.ts)
│   ├── eslint-config/          # Shared ESLint configs (base.js, react.js, next.js)
│   └── tsconfig/               # Shared tsconfig presets
├── apps/
│   ├── web/                    # Next.js App Router (app/, platform/, components/, features/, lib/, proxy.ts)
│   ├── desktop/                # Electron (src/main, src/preload, src/renderer/src, src/shared)
│   ├── mobile/                 # Expo/React Native — independent (app/, data/, components/, lib/, locales/)
│   ├── docs/                   # Fumadocs site (app/[lang], content/docs/*.mdx)
│   └── ui-lab/                 # Vite UI Lab for tokens/components (src/)
├── e2e/                        # Playwright specs (*.spec.ts), fixtures.ts, helpers.ts, perf/
├── examples/plugins/           # Example plugins (hello-panel, release-checklist, deploy-sentinel)
├── scripts/                    # Dev env, check, upstream-sync, reserved-slug generation, guards
├── docs/                       # Engineering notes (docs/engineering/, maintenance-jobs.md)
├── deploy/helm/multica/        # Helm chart
├── docker/                     # Container entrypoint
├── Makefile                    # All dev/check/release targets
├── docker-compose*.yml         # Local Postgres + self-hosted stack
├── Dockerfile, Dockerfile.web  # Backend and web images
├── pnpm-workspace.yaml         # Workspaces (apps/*, packages/*) + dependency catalog
├── turbo.json                  # Turborepo task graph
├── knip.jsonc                  # Dependency/boundary guard (mobile excluded)
├── playwright*.config.ts       # E2E configs
└── AGENTS.md                   # Authoritative repo rules (CLAUDE.md imports it)
```

## Directory Purposes

**`server/cmd/server/`:**
- Purpose: Composition root of the API process.
- Contains: `main.go`, `router.go` (all routes), `listeners.go` and `*_listeners.go` (event bus subscribers), `runtime_sweeper.go`, `source_context_sweeper.go`, `health.go`, `shutdown.go`, `scope_authorizer.go`.
- Key files: `server/cmd/server/main.go`, `server/cmd/server/router.go`, `server/cmd/server/listeners.go`

**`server/internal/handler/`:**
- Purpose: HTTP handlers on the shared `*Handler` receiver; ~131 non-test files.
- Contains: `<resource>.go` (e.g. `issue.go`, `label.go`, `agent.go`, `daemon.go`, `chat.go`, `inbox.go`, `plugin*.go`), tests `<resource>_test.go`, `reserved_slugs.json`.
- Key files: `server/internal/handler/handler.go` (struct, response/UUID helpers, loaders), `server/internal/handler/reserved_slugs.json`

**`server/internal/service/`:**
- Purpose: Transactional domain logic shared by handlers, listeners and scheduler.
- Contains: `issue.go`, `task.go`, `autopilot*.go`, `plugin*.go`, `issue_wakeup*.go`, `email.go`, `builtin_agents/`, `builtin_skills/`.
- Key files: `server/internal/service/task.go`, `server/internal/service/issue.go`

**`server/pkg/db/`:**
- Purpose: sqlc input and output.
- Contains: `queries/<entity>.sql` (65 files), `generated/<entity>.sql.go`, `generated/db.go`, hand-written row helpers like `generated/attachment_row.go`.
- Key files: `server/sqlc.yaml`

**`server/internal/daemon/` + `server/pkg/agent/`:**
- Purpose: Agent execution on user machines.
- Contains: daemon loop (`daemon.go`), HTTP client (`client.go`), WS RPC (`wsrpc.go`), config (`config.go`), exec environment prep (`execenv/`), git cache (`repocache/`); agent adapters per CLI with OS-specific splits (`*_windows.go`, `*_other.go`, `*_unix.go`).

**`packages/core/`:**
- Purpose: Headless client logic for web + desktop.
- Contains: `api/` (`client.ts`, `schema.ts`, `schemas.ts`, `ws-client.ts`), `platform/` (`core-provider.tsx`, `storage.ts`, `persist-storage.ts`, `workspace-storage.ts`), `realtime/` (`provider.tsx`, `hooks.ts`, `use-realtime-sync.ts`), `paths/` (`paths.ts`, `reserved-slugs.ts`), `types/` (`events.ts`, `storage.ts`, entity types), and feature dirs (`issues/`, `inbox/`, `projects/`, `agents/`, `chat/`, `labels/`, `workspace/`, …).
- Key files: `packages/core/package.json` (per-file `exports` map — new public modules must be added here)

**`packages/views/`:**
- Purpose: Shared pages/business components.
- Contains: feature dirs with `components/`, `hooks/`, `utils/`, an `index.ts`; `layout/` (dashboard shell, `dashboard-guard.tsx`, `app-sidebar.tsx`), `navigation/` (adapter contract), `platform/` (desktop-aware shared helpers like `drag-strip.tsx`), `modals/` (`registry.tsx`), `locales/{en,fr,ja,ko,zh-Hans}/`.
- Key files: `packages/views/package.json` (exports map), `packages/views/navigation/types.ts`, `packages/views/eslint.config.mjs` (i18n enforcement)

**`packages/ui/`:**
- Purpose: Design system.
- Contains: `components/ui/*.tsx` (shadcn/Base UI primitives), `components/common/*.tsx`, `styles/tokens.css`, `markdown/`, `hooks/`, `lib/utils.ts`, `docs/button.md`, `docs/dialog.md`.

**`apps/web/`:**
- Purpose: Next.js web client and landing site.
- Contains: `app/[workspaceSlug]/(dashboard)/<route>/page.tsx` (thin wrappers rendering views), `app/(auth)/`, `app/(landing)/`, integration callbacks (`app/slack/`, `app/lark/`, `app/telegram/`, `app/wecom/`, `app/dingtalk/`), `platform/` (navigation, titles, scroll restoration, issue-view URL sync), `components/web-providers.tsx`, `features/{auth,landing}/`, `content/use-cases/`, `proxy.ts` (legacy URL rewrites).

**`apps/desktop/`:**
- Purpose: Electron client.
- Contains: `src/main/` (window, updater, `daemon-manager.ts`, CLI bootstrap), `src/preload/index.ts` (`window.desktopAPI`), `src/shared/` (main↔renderer message types), `src/renderer/src/` (`App.tsx`, `routes.tsx`, `pages/*-page.tsx`, `components/`, `platform/`, `stores/`, `hooks/`).

**`apps/mobile/`:**
- Purpose: Independent iOS/Android client (rules in `apps/mobile/AGENTS.md`).
- Contains: `app/` Expo Router (`(auth)/`, `(app)/[workspace]/(tabs)/`, sheet routes like `new-issue-picker/`), `data/` (`api.ts`, `schemas.ts`, `query-client.ts`, `queries/`, `mutations/`, `realtime/`, `stores/`), `components/ui/` (generic RNR primitives) and `components/<domain>/`, `lib/`, `locales/{en,zh-Hans}/`, `docs/`.

**`e2e/`:**
- Purpose: Playwright end-to-end tests against a running stack.
- Contains: `<feature>.spec.ts`, `fixtures.ts`, `helpers.ts` (`TestApiClient` setup/teardown), `env.ts`, `perf/`.

## Key File Locations

**Entry Points:**
- `server/cmd/server/main.go`: API server
- `server/cmd/multica/main.go`: CLI root; `server/cmd/multica/cmd_daemon.go`: daemon command
- `server/cmd/migrate/`: migration runner
- `apps/web/app/layout.tsx`: web root layout; `apps/web/app/[workspaceSlug]/layout.tsx`: workspace shell
- `apps/desktop/src/main/index.ts`: Electron main; `apps/desktop/src/renderer/src/main.tsx`: renderer
- `apps/mobile/app/_layout.tsx`: mobile root; `apps/mobile/app/(app)/[workspace]/_layout.tsx`: workspace shell + sheet registration
- `apps/docs/app/[lang]/`: docs site; `apps/ui-lab/src/main.tsx`: UI Lab

**Configuration:**
- `Makefile`: dev/check/release targets (`make up`, `make check`, `make sqlc`, `make upstream-sync`, `make local-release`)
- `pnpm-workspace.yaml`: workspaces + `catalog:` versions
- `turbo.json`: task graph (`mdx` → `build`/`typecheck`)
- `knip.jsonc`: declared-dependency guard
- `server/sqlc.yaml`: sqlc generation
- `docker-compose.yml`, `docker-compose.selfhost.yml`, `docker-compose.selfhost.build.yml`: local stacks
- `.env.example`: env template (`.env`, `.env.make`, `.env.worktree` present — contents not read)
- `playwright.config.ts`, `playwright.perf.config.ts`, `playwright.iframe.config.ts`

**Core Logic:**
- `server/cmd/server/router.go`: route table
- `server/internal/handler/handler.go`: handler base
- `server/internal/service/task.go`: task queue lifecycle
- `server/internal/realtime/hub.go`: WS hub
- `packages/core/api/client.ts`: API client
- `packages/core/realtime/use-realtime-sync.ts`: WS → Query cache sync
- `packages/core/platform/core-provider.tsx`: client bootstrap

**Testing:**
- Go: `*_test.go` beside code; fixtures `server/internal/testutil/db.go`, `server/internal/testutil/http.go`; runner `scripts/test-go.sh`
- TS: `*.test.ts(x)` beside code; package test setup `packages/views/test/`, `packages/ui/test/`, `apps/web/test/`, `apps/desktop/test/`
- E2E: `e2e/*.spec.ts`

## Naming Conventions

**Files:**
- Go: `snake_case.go`, tests `snake_case_test.go`, OS splits `_windows.go` / `_unix.go` / `_other.go`: `server/pkg/agent/cursor_invocation_windows.go`
- Go handlers/services: named for the resource or flow: `server/internal/handler/issue_reaction.go`, `server/internal/service/issue_wakeup_join.go`
- Go CLI commands: `cmd_<noun>[_<sub>].go`: `server/cmd/multica/cmd_issue_label.go`
- SQL: `server/pkg/db/queries/<entity>.sql`; migrations `NNN_snake_name.up.sql` + `.down.sql`, numbered sequentially: `server/migrations/563_search_index_change_changed_at_index.up.sql`
- TypeScript: `kebab-case.ts(x)`, tests `kebab-case.test.ts(x)`: `packages/views/issues/components/board-card.tsx`
- Core feature modules use fixed role names: `queries.ts`, `mutations.ts`, `ws-updaters.ts`, `stores/<name>-store.ts`, `index.ts`: `packages/core/issues/`
- Desktop pages: `<entity>-detail-page.tsx`: `apps/desktop/src/renderer/src/pages/issue-detail-page.tsx`
- Web routes: Next.js conventions `page.tsx`, `layout.tsx`, `loading.tsx` under route groups `(dashboard)`, `(auth)`, `(landing)`
- Mobile routes: Expo Router `[param].tsx`, `_layout.tsx`, groups `(tabs)`
- Docs: `<slug>.mdx` plus locale suffix `<slug>.zh.mdx`, `.ja.mdx`, `.ko.mdx`, `.fr.mdx`: `apps/docs/content/docs/agents.ja.mdx`

**Directories:**
- Feature-first lowercase/kebab directories mirrored across layers: `packages/core/issue-statuses/`, `packages/views/issues/`, `apps/web/app/[workspaceSlug]/(dashboard)/issues/`, `apps/mobile/data/queries/issues.ts`
- Go packages are single lowercase words: `server/internal/issuestatus/`, `server/internal/daemonws/`

**Global routes:**
- New pre-workspace global routes use a single word or `/{noun}/{verb}`, never hyphenated root names; reserved in `server/internal/handler/reserved_slugs.json` (`AGENTS.md`).

## Where to Add New Code

**New REST endpoint (backend):**
- SQL: add queries to `server/pkg/db/queries/<entity>.sql`, run `make sqlc` → regenerates `server/pkg/db/generated/`
- Schema change: new pair `server/migrations/<next NNN>_<name>.up.sql` / `.down.sql`; no foreign keys; each `CREATE [UNIQUE] INDEX CONCURRENTLY` in its own file
- Handler: method on `*Handler` in `server/internal/handler/<entity>.go`; route in `server/cmd/server/router.go` inside the correct group (workspace member/role, `RequireHumanActor`, daemon)
- Multi-step/transactional logic: `server/internal/service/<entity>.go`
- Realtime: add constant to `server/pkg/protocol/events.go`, publish via `h.publish(...)`; mirror in `packages/core/types/events.ts` (`WSEventType` + `WSEventPayloadMap`)
- Side effects on events: subscriber in `server/cmd/server/*_listeners.go`
- Periodic work: `server/internal/scheduler/jobs_<name>.go`
- Tests: `server/internal/handler/<entity>_test.go` using `server/internal/testutil` fixtures

**New client feature (web + desktop):**
- API method: `packages/core/api/client.ts`; zod schema in `packages/core/api/schemas.ts`; parse with `parseWithFallback`; malformed-response test in `packages/core/api/schemas.test.ts`
- Server state: `packages/core/<feature>/queries.ts` (key factory with `wsId`), `mutations.ts`, `ws-updaters.ts`; export via `packages/core/<feature>/index.ts` and `packages/core/package.json` `exports`
- Client state: `packages/core/<feature>/stores/<name>-store.ts` (never in views)
- UI: `packages/views/<feature>/components/` (+ `hooks/`), translated strings in `packages/views/locales/<lang>/`; export via `packages/views/package.json`
- Web route: `apps/web/app/[workspaceSlug]/(dashboard)/<route>/page.tsx` rendering the view
- Desktop route: register in `apps/desktop/src/renderer/src/routes.tsx`; one-shot pre-workspace flows use `WindowOverlay` (`apps/desktop/src/renderer/src/stores/window-overlay-store.ts`)
- Path helpers: `packages/core/paths/paths.ts`

**New UI primitive:**
- Implementation: `packages/ui/components/ui/<name>.tsx` via `pnpm ui:add <component>`; compositions in `packages/views/`
- Tokens: `packages/ui/styles/tokens.css`

**New mobile feature:**
- Screens: `apps/mobile/app/(app)/[workspace]/...`; pickers/sheets registered in `apps/mobile/app/(app)/[workspace]/_layout.tsx` with `SHEET_OPTIONS`
- Data: `apps/mobile/data/queries/<feature>.ts`, `apps/mobile/data/mutations/`, `apps/mobile/data/realtime/<feature>-ws-updaters.ts`, schemas in `apps/mobile/data/schemas.ts`
- Components: `apps/mobile/components/<domain>/`; generic primitives only in `apps/mobile/components/ui/`
- Pure helpers: `apps/mobile/lib/`

**New agent CLI backend:**
- Implementation: `server/pkg/agent/<name>.go` registered in `agent.New` (`server/pkg/agent/agent.go`); command name in `scripts/agent-cli-command-names.txt`; daemon probing in `server/internal/daemon/agents_probe.go`

**New CLI command:**
- Implementation: `server/cmd/multica/cmd_<noun>.go` with test `cmd_<noun>_test.go`

**Utilities:**
- Go shared helpers: `server/internal/util/`
- TS headless helpers: `packages/core/<area>/` (no DOM); UI helpers: `packages/ui/lib/`

## Special Directories

**`server/pkg/db/generated/`:**
- Purpose: sqlc-generated Go (`// Code generated by sqlc. DO NOT EDIT.`)
- Generated: Yes (`make sqlc`, config `server/sqlc.yaml`)
- Committed: Yes

**`packages/core/paths/reserved-slugs.ts`:**
- Purpose: Workspace slugs that collide with top-level routes
- Generated: Yes, from `server/internal/handler/reserved_slugs.json` via `pnpm generate:reserved-slugs` (`scripts/generate-reserved-slugs.mjs`)
- Committed: Yes (commit both files together)

**`server/migrations/`:**
- Purpose: Ordered schema history; skipped conditional migrations still record in `schema_migrations`
- Generated: No
- Committed: Yes

**`apps/web/.source/`, `apps/docs/.source/`:**
- Purpose: fumadocs-mdx generated content index (turbo `mdx` task)
- Generated: Yes
- Committed: No (`apps/web/.gitignore`, `apps/docs/.gitignore`)

**`server/bin/`:**
- Purpose: Built `server`, `multica`, `migrate` binaries (`make build`); `~/.local/bin/multica` links here in `make local-release`
- Generated: Yes
- Committed: No (`.gitignore`)

**`apps/mobile/ios/`, `apps/mobile/android/`:**
- Purpose: Expo prebuild native projects
- Generated: Yes (`apps/mobile/scripts/ios-run.sh` prebuilds)
- Committed: No

**`.next/`, `dist/`, `out/`, `dist-electron/`, `node_modules/`:**
- Purpose: Build outputs and dependencies
- Generated: Yes
- Committed: No (`.gitignore`)

**`.planning/`, `.omp/`, `.gsd/`:**
- Purpose: GSD planning artifacts and agent tooling (not product code)
- Generated: By tooling
- Committed: `.planning/` per workflow; `.gsd/` self-ignored (`.gsd/.gitignore`)

---

*Structure analysis: 2026-10-12*
