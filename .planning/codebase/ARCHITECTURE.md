---
last_mapped_commit: 80fb7a97dc7c0b6e79ccfd92174fd7e124541d09
last_mapped_at: 2026-10-12
---
<!-- refreshed: 2026-10-12 -->

# Architecture

**Analysis Date:** 2026-10-12

## System Overview

```text
┌───────────────────────────────────────────────────────────────────────────────┐
│                               Client applications                              │
├──────────────────┬──────────────────┬──────────────────┬──────────────────────┤
│   Web (Next.js)  │ Desktop (Electron)│ Mobile (Expo/RN) │ CLI + daemon (Go)    │
│   `apps/web/`    │  `apps/desktop/` │  `apps/mobile/`  │ `server/cmd/multica/`│
│                  │                  │  (independent)   │ `server/internal/    │
│                  │                  │                  │   daemon/`           │
└────────┬─────────┴────────┬─────────┴────────┬─────────┴──────────┬───────────┘
         │ uses             │ uses             │ type-only + pure   │ HTTP + WS
         ▼                  ▼                  │ utils from core    │ (/api/daemon/*)
┌──────────────────────────────────────┐       │                    │
│  `packages/views/` shared pages      │       │                    │
│  (NavigationAdapter, AppLink)        │       │                    │
├──────────────────┬───────────────────┤       │                    │
│ `packages/core/` │ `packages/ui/`    │◄──────┘                    │
│ API client, zod, │ primitives,       │                            │
│ Query, Zustand,  │ tokens, markdown  │                            │
│ WS sync          │                   │                            │
└────────┬─────────┴───────────────────┘                            │
         │ REST (`/api/*`) + WebSocket (`/ws`)                      │
         ▼                                                          ▼
┌───────────────────────────────────────────────────────────────────────────────┐
│                 Go backend `server/cmd/server/` (Chi router)                   │
│  middleware `server/internal/middleware/` → handlers `server/internal/handler/`│
│  → services `server/internal/service/` → sqlc `server/pkg/db/generated/`       │
│  events bus `server/internal/events/` → listeners `server/cmd/server/*_listeners.go`
│  → realtime hub `server/internal/realtime/` (+ Redis relay) ; daemon hub       │
│  `server/internal/daemonws/` ; scheduler `server/internal/scheduler/`          │
└──────────────────────────────────┬────────────────────────────────────────────┘
                                   ▼
┌───────────────────────────────────────────────────────────────────────────────┐
│ PostgreSQL (schema `server/migrations/`), optional Redis, object storage       │
│ (`server/internal/storage/`), agent CLIs launched by daemon (`server/pkg/agent/`)│
└───────────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| API server entry | Boots config, DB pool, hub, event bus, listeners, scheduler, HTTP servers | `server/cmd/server/main.go` |
| Router | Global middleware, every REST/WS route, auth/workspace groups | `server/cmd/server/router.go` |
| Handlers | Decode request, resolve workspace/actor, call sqlc or service, publish event, write JSON | `server/internal/handler/*.go` (base in `server/internal/handler/handler.go`) |
| Services | Multi-step/transactional domain logic (issues, tasks, autopilots, plugins, wakeups) | `server/internal/service/issue.go`, `server/internal/service/task.go`, `server/internal/service/autopilot.go`, `server/internal/service/plugin.go` |
| Data access | sqlc-generated typed queries from SQL files | `server/pkg/db/queries/*.sql` → `server/pkg/db/generated/*.sql.go` |
| Event bus | In-process synchronous pub/sub of domain events | `server/internal/events/bus.go` |
| Listeners | Fan events to WS, activity log, notifications, subscribers, autopilots | `server/cmd/server/listeners.go`, `server/cmd/server/activity_listeners.go`, `server/cmd/server/notification_listeners.go`, `server/cmd/server/subscriber_listeners.go`, `server/cmd/server/autopilot_listeners.go` |
| Client realtime hub | WebSocket rooms per workspace/user/task/chat scope; Redis stream relay for multi-node | `server/internal/realtime/hub.go`, `server/internal/realtime/redis_relay.go`, `server/internal/realtime/sharded_stream_relay.go` |
| Daemon WS hub | Push wakeups/hints to connected daemons | `server/internal/daemonws/hub.go`, `server/internal/daemonws/notifier.go` |
| Event contract | WS event type constants shared by server/daemon | `server/pkg/protocol/events.go`, `server/pkg/eventcontract/events.go` |
| Scheduler | DB-backed periodic jobs (autopilots, wakeups, search index, usage) | `server/internal/scheduler/manager.go`, `server/internal/scheduler/jobs_*.go` |
| CLI | Cobra CLI (`multica`), includes `daemon` subcommand | `server/cmd/multica/main.go`, `server/cmd/multica/cmd_daemon.go` |
| Daemon runtime | Registers local runtimes, claims tasks, runs agent CLIs, reports results | `server/internal/daemon/daemon.go` |
| Agent backends | Per-CLI adapters (Claude, Codex, Cursor, OpenCode, …) behind `Backend` | `server/pkg/agent/agent.go`, `server/pkg/agent/claude.go`, `server/pkg/agent/codex.go` |
| Public plugin API | Versioned `/v1` contract + OpenAPI | `server/pkg/publicapi/v1/routes.go`, `server/pkg/publicapi/v1/openapi.yaml` |
| Core (headless) | API client, zod schemas, Query keys/options/mutations, Zustand stores, WS sync | `packages/core/api/client.ts`, `packages/core/api/schema.ts`, `packages/core/realtime/use-realtime-sync.ts`, `packages/core/platform/core-provider.tsx` |
| UI | shadcn/Base UI primitives, tokens, markdown | `packages/ui/components/ui/`, `packages/ui/styles/tokens.css`, `packages/ui/markdown/` |
| Views | Shared web/desktop pages and business components | `packages/views/issues/components/`, `packages/views/layout/dashboard-layout.tsx` |
| Web app | Next.js App Router routes and web adapters | `apps/web/app/[workspaceSlug]/(dashboard)/`, `apps/web/platform/navigation.tsx`, `apps/web/components/web-providers.tsx` |
| Desktop app | Electron main/preload + react-router renderer, tabs, daemon manager | `apps/desktop/src/main/index.ts`, `apps/desktop/src/preload/index.ts`, `apps/desktop/src/renderer/src/routes.tsx` |
| Mobile app | Independent Expo Router client with own data layer | `apps/mobile/app/`, `apps/mobile/data/api.ts`, `apps/mobile/data/realtime/` |

## Pattern Overview

**Overall:** Layered monolith backend (handler → service → sqlc) with an in-process event bus driving WebSocket fan-out; headless-core + shared-views frontend monorepo injected with platform adapters; a separate polling/WS daemon that executes agent CLIs on user machines.

**Key Characteristics:**
- Server is the single source of truth; clients hold server state only in TanStack Query caches and patch/invalidate them from WS events (`packages/core/realtime/use-realtime-sync.ts`).
- Dependency direction on the frontend is `views → core + ui`; `core` and `ui` never import each other (`AGENTS.md` "Sharing Rules and Package Boundaries"). Apps sit above views and supply platform adapters.
- Shared packages export raw TypeScript compiled by the consuming app via per-file `exports` maps (`packages/views/package.json`, `packages/core/package.json`, `packages/ui/package.json`).
- Workspace identity is route-driven (`/{workspaceSlug}/...`) and mirrored into a module-level singleton for request headers (`packages/core/platform/workspace-storage.ts`).
- Mobile is a peer client, not a consumer of `core` runtime: it imports core types with `import type` and only pure utilities/schemas (`apps/mobile/AGENTS.md`).

## Layers

**HTTP routing & middleware (Go):**
- Purpose: Request IDs, logging, recovery, CSP, CORS, auth (session/PAT/daemon/plugin tokens), workspace membership/role gates, rate limits.
- Location: `server/cmd/server/router.go`, `server/internal/middleware/`
- Contains: `middleware.Auth` (`server/internal/middleware/auth.go`), `middleware.DaemonAuth` (`server/internal/middleware/daemon_auth.go`), `RequireWorkspaceMemberFromURL` / `RequireWorkspaceRole` / `ResolveWorkspaceIDFromRequest` (`server/internal/middleware/workspace.go`), `PluginRateLimit` (`server/internal/middleware/plugin_ratelimit.go`).
- Depends on: `server/pkg/db/generated`, `server/internal/auth`
- Used by: every route group in `server/cmd/server/router.go`

**Handlers (Go):**
- Purpose: Thin HTTP adapters; one file per resource.
- Location: `server/internal/handler/`
- Contains: `Handler` struct with `Queries`, `TxStarter`, `Hub`, `DaemonHub`, `Bus`, services (`server/internal/handler/handler.go:203`); helpers `writeJSON`, `writeError`, `writeErrorCode`, `parseUUIDOrBadRequest`, `resolveWorkspaceID`, `resolveActor`, `loadIssueForUser`, `publish`.
- Depends on: services, sqlc queries, events bus, `server/pkg/protocol`
- Used by: router

**Services (Go):**
- Purpose: Domain workflows needing transactions, cross-entity rules, task enqueueing, analytics.
- Location: `server/internal/service/`
- Contains: `IssueService` (`server/internal/service/issue.go:35`), `TaskService` (`server/internal/service/task.go:39`), `AutopilotService` (`server/internal/service/autopilot.go:36`), `PluginService` (`server/internal/service/plugin.go:29`), `IssueWakeupService` (`server/internal/service/issue_wakeup.go:105`), `EmailService`.
- Depends on: `*db.Queries`, `TxStarter`, `*events.Bus`, `*realtime.Hub`
- Used by: handlers, listeners, scheduler jobs

**Persistence (Go):**
- Purpose: Typed SQL access; schema via numbered migrations.
- Location: `server/pkg/db/queries/` (hand-written SQL), `server/pkg/db/generated/` (sqlc output, `// Code generated by sqlc. DO NOT EDIT.`), `server/migrations/` (`NNN_name.up.sql`/`.down.sql`), config `server/sqlc.yaml`.
- Depends on: `pgx/v5`
- Used by: handlers, services, middleware, listeners

**Domain packages (Go):**
- Purpose: Focused subsystems kept out of handler/service files.
- Location: `server/internal/issuestatus/`, `server/internal/issueproperty/`, `server/internal/issueposition/`, `server/internal/issueguard/`, `server/internal/entitlement/`, `server/internal/seatcapacity/`, `server/internal/integrations/{slack,lark,telegram,wecom,dingtalk,vcs,composio}/`, `server/internal/storage/`, `server/internal/skill/`, `server/internal/featureflags/`.

**Daemon / agent runtime (Go):**
- Purpose: Runs on the user's machine; registers runtimes per workspace, claims queued tasks, prepares exec environments/repos, launches agent CLIs, streams messages, reports completion.
- Location: `server/internal/daemon/` (`daemon.go`, `client.go`, `wsrpc.go`, `execenv/`, `repocache/`, `processtree/`), `server/pkg/agent/` (one file per agent CLI).
- Depends on: server daemon API (`/api/daemon/*` in `server/cmd/server/router.go:1555`), `server/pkg/protocol`
- Used by: `server/cmd/multica/cmd_daemon.go`; the desktop app supervises it via `apps/desktop/src/main/daemon-manager.ts`.

**Core (TS, headless):**
- Purpose: All client logic shared by web and desktop — API client, response validation, Query hooks, Zustand client state, WS client and cache sync, paths, i18n.
- Location: `packages/core/`
- Contains: `ApiClient` (`packages/core/api/client.ts:757`), schemas (`packages/core/api/schemas.ts`), `parseWithFallback` (`packages/core/api/schema.ts:38`), `WSClient` (`packages/core/api/ws-client.ts:35`), feature dirs with `queries.ts` / `mutations.ts` / `ws-updaters.ts` / `stores/` (e.g. `packages/core/issues/`), `CoreProvider` (`packages/core/platform/core-provider.tsx:140`).
- Depends on: zod, TanStack Query, Zustand, i18next. No `react-dom`, `localStorage`, `process.env`, or UI libraries.
- Used by: `packages/views/`, `apps/web/`, `apps/desktop/`

**UI (TS):**
- Purpose: Presentational primitives and design tokens; no business logic, no `@multica/core` imports.
- Location: `packages/ui/` (`components/ui/`, `components/common/`, `styles/`, `markdown/`, `hooks/`, `docs/`)
- Used by: `packages/views/`, apps

**Views (TS):**
- Purpose: Shared pages and business components rendered identically by web and desktop.
- Location: `packages/views/` (feature dirs: `issues/`, `projects/`, `agents/`, `chat/`, `inbox/`, `settings/`, `layout/`, `navigation/`, `modals/`, `editor/`, `locales/`)
- Depends on: `@multica/core`, `@multica/ui`. Never `next/*` or `react-router-dom`; navigation goes through `useNavigation()` / `<AppLink>` (`packages/views/navigation/`). Must not define stores.
- Used by: `apps/web/`, `apps/desktop/`

**Apps (TS):**
- Purpose: Framework shells; route files map URLs to views; platform adapters for navigation, storage, titles, OS integration.
- Location: `apps/web/` (Next.js App Router, adapters in `apps/web/platform/`), `apps/desktop/src/renderer/src/` (react-router memory router, adapters in `apps/desktop/src/renderer/src/platform/`, desktop-only stores in `apps/desktop/src/renderer/src/stores/`).

**Mobile (TS, independent):**
- Purpose: Phone client with own UI, state, Query client, WS client, i18n and build.
- Location: `apps/mobile/` (`app/` Expo Router, `data/` API/queries/mutations/realtime/stores, `components/`, `lib/`, `locales/`)
- Depends on: `@multica/core` types (type-only) and pure utilities/schemas only.

## Data Flow

### Primary Request Path (web/desktop mutation)

1. A view calls a core mutation hook, e.g. `packages/core/issues/mutations.ts` (`onMutate` at line 200 snapshots + patches for status/priority only).
2. `ApiClient.fetch` adds auth and `X-Workspace-Slug` from the current workspace singleton (`packages/core/api/client.ts:830`, `packages/core/platform/workspace-storage.ts:34`).
3. Chi global middleware runs, then `middleware.Auth` and workspace gates (`server/cmd/server/router.go:1406-1427`, `server/internal/middleware/workspace.go:68`).
4. Handler decodes, validates, resolves IDs (`server/internal/handler/label.go:192` `CreateLabel` is the canonical small example): `h.resolveWorkspaceID(r)` → `h.Queries.CreateLabel(...)` (sqlc) → `h.publish(protocol.EventLabelCreated, ...)` → `writeJSON`.
5. Complex writes delegate to services in a transaction, e.g. `IssueService.Create` (`server/internal/service/issue.go:235`), which also enqueues agent tasks via `TaskService`.
6. Client parses the response through zod via `parseWithFallback` (`packages/core/api/schema.ts:38`) and settles/invalidates its Query cache.

### Realtime Fan-out and Cache Patching

1. `h.publish` / services call `events.Bus.Publish` (`server/internal/events/bus.go:73`); type-specific subscribers run first, then `SubscribeAll` handlers, synchronously with panic recovery.
2. `registerListeners` (`server/cmd/server/listeners.go:79`, wired at `server/cmd/server/main.go:605`) projects payloads (`projectOutbound`) and sends to a workspace room, a user, or task/chat scopes through `realtime.Broadcaster`.
3. `realtime.Hub` (`server/internal/realtime/hub.go:277`) writes to subscribed WS clients; in multi-node deployments the sharded Redis stream relay (`server/internal/realtime/sharded_stream_relay.go`, mode `REALTIME_RELAY_MODE` in `server/cmd/server/main.go:112`) replicates events across nodes.
4. Browser `WSClient` (`packages/core/api/ws-client.ts`) receives events; `useRealtimeSync` (`packages/core/realtime/use-realtime-sync.ts:768`) maps event prefixes (`issue`, `inbox`, `agent`, `project`, `label`, `daemon`, …) to debounced invalidations, and feature updaters patch caches when the payload is determinate (`packages/core/issues/ws-updaters.ts` `onIssueCreated`, `onIssueUpdated`, `onIssueDeleted`).
5. Page-level granular subscriptions use `useWSEvent` (`packages/core/realtime/hooks.ts:13`).
6. Mobile mirrors this with its own updaters (`apps/mobile/data/realtime/issue-ws-updaters.ts`, `apps/mobile/data/realtime/inbox-ws-updaters.ts`) and `RealtimeSubscriptions` in `apps/mobile/app/(app)/[workspace]/_layout.tsx`.

### Agent Task Execution

1. A trigger (assignment, mention, comment, autopilot, wakeup) enqueues a row in `agent_task_queue` via `TaskService` (`server/internal/service/task.go`) and emits `task:queued` (`server/pkg/protocol/events.go`).
2. `daemonws` pushes a pending-work hint to the owning daemon (`server/internal/daemonws/notifier.go`; `DaemonPendingWorkNotifier` in `server/internal/handler/handler.go:179`).
3. Daemon `pollLoop` (`server/internal/daemon/daemon.go:5334`) claims via `POST /api/daemon/runtimes/{runtimeId}/tasks/claim` (`server/cmd/server/router.go:1574`).
4. `handleTask` → `runTask` (`server/internal/daemon/daemon.go:5707`, `:7649`) prepares the workdir (`server/internal/daemon/execenv/`, `server/internal/daemon/repocache/`) and launches the agent CLI through `agent.New(agentType, cfg)` (`server/pkg/agent/agent.go:432`).
5. Daemon streams `/tasks/{taskId}/messages` and `/progress`, then `/complete` or `/fail` (`server/cmd/server/router.go:1589-1598`); server publishes `task:*` events that clients invalidate.

### Scheduled Work

1. `scheduler.NewManager(pool, ...)` (`server/cmd/server/main.go:818`) ticks registered jobs recorded in `sys_cron_executions` (`server/internal/scheduler/manager.go:43`).
2. Jobs live in `server/internal/scheduler/jobs_autopilot.go`, `jobs_issue_wakeup.go`, `jobs_search_index.go`, `jobs_task_usage.go`, `jobs_plugin_hook.go`, `jobs_child_done.go`.
3. Sweepers run beside it: `server/cmd/server/runtime_sweeper.go`, `server/cmd/server/source_context_sweeper.go`.

**State Management:**
- Server state: TanStack Query only; workspace-scoped keys include `wsId` (`packages/core/issues/queries.ts:45` `issueKeys.all(wsId)`).
- Client state: Zustand stores in `packages/core/*/stores/` (e.g. `packages/core/issues/stores/draft-store.ts`); desktop-only stores in `apps/desktop/src/renderer/src/stores/tab-store.ts` and `window-overlay-store.ts`; mobile stores in `apps/mobile/data/stores/`.
- Persistence: only durable preferences/drafts/layout, via `StorageAdapter` (`packages/core/platform/persist-storage.ts`, `packages/core/platform/storage.ts`).
- React Context is used for platform plumbing only (navigation, workspace slug, view-store context `packages/core/issues/stores/view-store-context.tsx`).

## Key Abstractions

**NavigationAdapter:**
- Purpose: Platform-neutral router contract (`push`, `replace`, `back`, `pathname`, `searchParams`, `hash`, optional `openInNewTab`, `getShareableUrl`, `prefetch`, `canGoBack`, `forward`).
- Examples: interface `packages/views/navigation/types.ts:1`; provider `packages/views/navigation/context.tsx`; implementations `apps/web/platform/navigation.tsx` (`WebNavigationProvider`), `apps/desktop/src/renderer/src/platform/navigation.tsx` (`DesktopNavigationProvider`), `apps/desktop/src/renderer/src/platform/issue-window-navigation.tsx`.
- Pattern: Adapter + React context; consumers use `useNavigation()`, `<AppLink>` (`packages/views/navigation/app-link.tsx`), `useBackOrReplace()`.

**StorageAdapter:**
- Purpose: Synchronous key/value persistence injected into core so core never touches `localStorage`.
- Examples: interface `packages/core/types/storage.ts:1`; default and workspace-namespaced wrappers `packages/core/platform/storage.ts`, `packages/core/platform/persist-storage.ts`, `packages/core/platform/storage-cleanup.ts`; passed via `CoreProvider storage` prop (`packages/core/platform/core-provider.tsx:144`).
- Pattern: Dependency injection at app bootstrap.

**parseWithFallback:**
- Purpose: API-boundary compatibility; validate UI-consumed JSON with zod, log schema drift, return a typed fallback instead of crashing older desktop clients talking to newer servers.
- Examples: `packages/core/api/schema.ts:38`; schemas in `packages/core/api/schemas.ts`; tests `packages/core/api/schemas.test.ts`; mobile equivalent `apps/mobile/lib/parse-response.ts` + `apps/mobile/data/schemas.ts`.
- Pattern: `parseWithFallback(data, Schema, fallback, { endpoint })` — never `as T`.

**CoreProvider:**
- Purpose: Initialize API client, auth/chat stores, session renewal, WS provider, i18n once per process.
- Examples: `packages/core/platform/core-provider.tsx`; mounted by `apps/web/components/web-providers.tsx:71` and `apps/desktop/src/renderer/src/App.tsx:459`.
- Pattern: Module-level singletons created on first render.

**Query key factories:**
- Purpose: Hierarchical, invalidatable keys per feature.
- Examples: `issueKeys` (`packages/core/issues/queries.ts:45`), `workspaceKeys` (`packages/core/workspace/queries.ts`), mobile `apps/mobile/data/queries/issue-keys.ts`.
- Pattern: `all(wsId)` prefix → specific full keys; mutations import the same factory.

**Event bus + Event:**
- Purpose: Decouple writes from side effects (WS, activity, notifications, subscribers, plugins, autopilots).
- Examples: `server/internal/events/bus.go`, event constants `server/pkg/protocol/events.go`, TS mirror `packages/core/types/events.ts` (`WSEventType`, `WSEventPayloadMap`).
- Pattern: Synchronous in-process pub/sub; `Event{Type, WorkspaceID, ActorType, ActorID, Payload, TaskID, ChatSessionID}`.

**Agent Backend:**
- Purpose: Uniform interface over many agent CLIs.
- Examples: `server/pkg/agent/agent.go:18` (`Backend`), factory `agent.New` at `server/pkg/agent/agent.go:432`, implementations `server/pkg/agent/{claude,codex,cursor,opencode,copilot,hermes,kimi,qwen,...}.go`.
- Pattern: Strategy/factory keyed by agent type.

**Plugins:**
- Purpose: Third-party surfaces, actions, hooks, MCP tools, storage.
- Examples: SDK `packages/plugin-sdk/index.ts`, `packages/plugin-sdk/protocol.ts`; contract `server/pkg/plugincontract/`; services `server/internal/service/plugin*.go`; handlers `server/internal/handler/plugin*.go`; examples `examples/plugins/hello-panel/`.

## Entry Points

**API server:**
- Location: `server/cmd/server/main.go` (`main` at line 327)
- Triggers: `make server`, `make start`, Docker image `Dockerfile`
- Responsibilities: Env validation, DB pool, `hub.Run()` (line 430), listener registration (lines 605–617, 728), `NewRouterWithOptions` (line 682), scheduler (line 818), main/metrics/maintenance/profiling servers, graceful shutdown (`server/cmd/server/shutdown.go`).

**CLI / daemon:**
- Location: `server/cmd/multica/main.go` (Cobra `rootCmd`), daemon command `server/cmd/multica/cmd_daemon.go`, runtime `server/internal/daemon/daemon.go:2075` (`Run`).
- Triggers: `multica ...`, `make daemon`, desktop `apps/desktop/src/main/daemon-manager.ts`.

**Migrations / maintenance:**
- Location: `server/cmd/migrate/`, `server/cmd/maintenance/`, one-off backfills `server/cmd/backfill_*`.
- Triggers: `make migrate-up`, `make migrate-down`.

**Web app:**
- Location: `apps/web/app/layout.tsx`, providers `apps/web/components/web-providers.tsx`, workspace shell `apps/web/app/[workspaceSlug]/layout.tsx`, legacy-URL rewriting `apps/web/proxy.ts`.

**Desktop app:**
- Location: main `apps/desktop/src/main/index.ts`, preload `apps/desktop/src/preload/index.ts`, renderer `apps/desktop/src/renderer/src/main.tsx` → `App.tsx` → `routes.tsx`.

**Mobile app:**
- Location: `apps/mobile/app/_layout.tsx`, workspace shell `apps/mobile/app/(app)/[workspace]/_layout.tsx`.

**Docs / UI Lab:**
- Location: `apps/docs/app/[lang]/`, `apps/ui-lab/src/main.tsx`.

## Architectural Constraints

- **Threading:** Go server uses goroutines per request plus long-lived goroutines (`hub.Run`, scheduler, sweepers, relays). The event bus is synchronous: a slow subscriber blocks the publishing request (`server/internal/events/bus.go:69-73`). JS clients are single-threaded; Electron splits main (`apps/desktop/src/main/`) and renderer processes communicating over preload IPC (`apps/desktop/src/shared/main-renderer-messages.ts`).
- **Global state:** `packages/core/platform/core-provider.tsx` (module-level `authStore`, `chatStore`, `sessionRenewal`); `packages/core/platform/workspace-storage.ts` (current slug/wsId singleton read by `getCurrentWsId()`); desktop `singletonOwner` in `apps/desktop/src/renderer/src/components/workspace-route-layout.tsx`; Go package-level allowed origins/trusted proxies in `server/internal/realtime/hub.go:108-118`.
- **Package boundaries:** `views → core + ui` only; `core` has no UI libs/`react-dom`/`localStorage`/`process.env`; `ui` has no `@multica/core`; views have no `next/*`/`react-router-dom` (`AGENTS.md`). Undeclared imports are caught by knip (`knip.jsonc`); `apps/mobile` is excluded from knip and root checks.
- **API compatibility:** Every UI-consumed response passes zod + `parseWithFallback`; update schema + malformed-response tests when changing an endpoint (`AGENTS.md` "API Compatibility").
- **Database:** No foreign keys or cascades; enforce relationships in application code with transactions. Every migration index uses `CREATE [UNIQUE] INDEX CONCURRENTLY`, one per migration file (`server/migrations/562_search_index_change_workspace_index.up.sql` pattern). Conditional DDL must be idempotent.
- **UUIDs in handlers:** Use `parseUUIDOrBadRequest` for raw input, loaders (`loadIssueForUser`, `loadAgentForUser`, …) for UUID-or-identifier params, `parseUUID` only for trusted sqlc round-trips (`server/internal/handler/handler.go:619-706`).
- **Workspace scoping:** Queries filter by `workspace_id`; membership gates access and `X-Workspace-ID`/`X-Workspace-Slug` selects the workspace (`server/internal/middleware/workspace.go:68`). Task tokens (`X-Actor-Source: task_token`) are pinned to their workspace.
- **Circular imports:** None detected; Go `internal/handler` depends on `internal/service`, never the reverse.
- **Fork constraints:** No remote CI; `make check` runs locally; updates go through `make upstream-sync` / `make local-release` (`Makefile:386-391`).

## Anti-Patterns

### Storing server data in Zustand

**What happens:** Copying API payloads into a store to share between components.
**Why it's wrong:** Creates a second source of truth that WS events never update (`AGENTS.md` "State Rules").
**Do this instead:** Read via Query options (`packages/core/issues/queries.ts`) and patch caches from WS updaters (`packages/core/issues/ws-updaters.ts`). Only auth/workspace stores may call `api.*` directly.

### Casting API responses with `as T`

**What happens:** `const data = (await res.json()) as Issue`.
**Why it's wrong:** Newer servers break installed desktop clients on unknown enums/missing fields.
**Do this instead:** `parseWithFallback(data, IssueSchema, fallback, { endpoint })` from `packages/core/api/schema.ts`.

### Framework imports in shared views

**What happens:** `import { useRouter } from "next/navigation"` inside `packages/views/`.
**Why it's wrong:** Breaks the desktop renderer (react-router memory router).
**Do this instead:** `useNavigation()` / `<AppLink>` from `packages/views/navigation/`; wire platform behavior in `apps/web/platform/` or `apps/desktop/src/renderer/src/platform/`.

### Navigating before the server confirms destructive/leave flows

**What happens:** Clearing/navigating first, then calling leave — as in `packages/views/settings/components/workspace-tab.tsx` (known `member:removed` race debt).
**Why it's wrong:** Races with WS events and optimistic deletes leave inconsistent UI.
**Do this instead:** Await the server, then navigate; use `switchWorkspace(slug, targetPath)` for cross-workspace navigation and `setCurrentWorkspace` from `@multica/core/platform` in route layouts.

### Feeding raw request strings to `parseUUID`

**What happens:** `parseUUID(chi.URLParam(r, "id"))` in a write path.
**Why it's wrong:** Panics or writes zero UUIDs (silent data loss, #1661).
**Do this instead:** `parseUUIDOrBadRequest(w, s, "id")` or a `load*ForUser` loader (`server/internal/handler/handler.go:686`).

### Mobile importing web/desktop runtime code

**What happens:** Importing core Query factories, stores or WS updaters into `apps/mobile/`.
**Why it's wrong:** Mobile owns its caches, QueryClient and React version.
**Do this instead:** `import type` from `@multica/core`; mirror behavior in `apps/mobile/data/queries/` and `apps/mobile/data/realtime/`.

## Error Handling

**Strategy:** Go handlers return early with JSON `{"error": msg}` (+ optional `code`) via `writeError` / `writeErrorCode` / `writeFeatureDisabled` / `writeRevisionConflict` (`server/internal/handler/handler.go:580-617`); Chi `Recoverer` turns panics into 500. Postgres errors are classified (`isNotFound`, `isUniqueViolation`, `isCheckViolation`). Frontend throws `ApiError` (`packages/core/api/client.ts:536`) with `errorCode()` for translatable messages.

**Patterns:**
- Map known DB failures to 4xx (`server/internal/handler/label.go:227` unique violation → 409); log others with `slog.Warn(..., logger.RequestAttrs(r)...)` and return a generic 500.
- Event bus recovers subscriber panics individually (`server/internal/events/bus.go:71`).
- Optimistic mutations snapshot before patching, roll back on error, invalidate on settle (`packages/core/issues/mutations.ts:200`).
- Schema drift is logged, not thrown (`packages/core/api/schema.ts:46`).

## Cross-Cutting Concerns

**Logging:** `log/slog` with request attrs (`server/internal/logger/`, `server/internal/middleware/request_logger.go`); frontend schema logger in `packages/core/api/schema.ts`.
**Validation:** Go handler-level validation helpers per resource (e.g. `validateLabelName` in `server/internal/handler/label.go`); DB CHECK constraints surfaced as 4xx; zod on the client.
**Authentication:** Cookie sessions (web), bearer tokens (desktop/mobile/CLI), PATs, daemon tokens, `mat_` task tokens, plugin bearer tokens (`server/internal/middleware/auth.go`, `daemon_auth.go`, `plugin_auth.go`, `server/internal/auth/`); WS auth via first-message token (`server/internal/realtime/hub.go:724`).
**Authorization:** Workspace membership/role middleware (`server/internal/middleware/workspace.go`), actor resolution (`resolveActor`, `server/internal/handler/handler.go:864`), `RequireHumanActor` groups in `server/cmd/server/router.go`.
**i18n:** Views must route JSX text through `useT()` (enforced by `packages/views/eslint.config.mjs`); locales in `packages/views/locales/{en,fr,ja,ko,zh-Hans}/`; mobile `apps/mobile/locales/`.
**Metrics/observability:** `server/internal/metrics/`, `/health`, `/readyz`, `/health/realtime` (`server/cmd/server/router.go:1434-1447`), `server/internal/profiling/`.

---

*Architecture analysis: 2026-10-12*
