---
last_mapped_commit: 80fb7a97dc7c0b6e79ccfd92174fd7e124541d09
last_mapped_at: 2026-10-12
---
# Coding Conventions

**Analysis Date:** 2026-10-12

Authoritative sources, in precedence order:

1. `AGENTS.md` (repo root) — package boundaries, state, API compatibility, DB, UUID, UI and testing rules. `CLAUDE.md` only imports it.
2. `apps/mobile/AGENTS.md` — additional rules for everything under `apps/mobile/`.
3. `apps/docs/content/docs/developers/conventions.mdx` (and `conventions.zh.mdx`) — naming, i18n glossary, Chinese voice. The page states: "This page is the contract; nothing else overrides it."
4. `packages/ui/docs/button.md`, `packages/ui/docs/dialog.md` — component contracts that also feed UI Lab (`apps/ui-lab/`).

Fork-specific (`AGENTS.md` § Fork): `origin` is `appboypov/multica`; `upstream` (`multica-ai/multica`) is fetch-only; there are no remote pipelines, so every check runs locally (`make check`, `scripts/check.sh`).

## Naming Patterns

**Files:**
- TypeScript files are `kebab-case.ts` / `kebab-case.tsx`: `packages/views/settings/components/telegram-tab.tsx`, `packages/core/issues/stores/create-mode-store.ts`.
- Zustand store files are `<feature>-store.ts` (`packages/core/issues/stores/draft-store.ts`, `packages/core/inbox/filter-store.ts`).
- TanStack Query modules are fixed names per feature: `queries.ts`, `mutations.ts`, `ws-updaters.ts` (`packages/core/issues/queries.ts`, `packages/core/labels/mutations.ts`, `packages/core/inbox/ws-updaters.ts`).
- Tests are colocated `<file>.test.ts(x)` (`packages/core/issues/queries.test.ts` beside `queries.ts`).
- Go files are `snake_case.go`; handler files mirror the domain (`server/internal/handler/agent.go`, `auth.go`, `runtime.go`); Go tests are colocated `<file>_test.go`. Regression tests may carry a ticket suffix (`server/internal/handler/admission_security_mul4525_test.go`).
- Go internal packages are single lowercase words with no underscores: `server/internal/issuestatus/`, `server/internal/dbstartup/`, `server/internal/testutil/`.
- Migrations are `NNN_descriptive_name.up.sql` + `NNN_descriptive_name.down.sql`, always both (`server/migrations/562_search_index_change_workspace_index.up.sql` / `.down.sql`).
- sqlc query files are `server/pkg/db/queries/<entity>.sql` with `-- name: GetIssue :one` annotations (`server/pkg/db/queries/issue.sql`).

**Functions:**
- TS functions and variables are `camelCase`. Hooks are `useCamelCase` (`useWorkspaceId`, `useCreateLabel` in `packages/core/labels/mutations.ts`).
- Query option factories are `<thing>Options` (`issueTasksOptions`, `memberListOptions`); query key factories are `<feature>Keys` objects (`issueKeys`, `labelKeys`, `inboxKeys`).
- WebSocket cache updaters are `on<Event>` / `patch<Thing>` (`onInboxIssueDeleted`, `patchInboxIssueProjection` in `packages/core/inbox/ws-updaters.ts`).
- Go exported identifiers are `PascalCase`, unexported `camelCase`; handlers are methods on `*Handler` (`func (h *Handler) ListInbox`). Loader helpers that write the error response themselves return `(T, bool)`: `loadIssueForUser`, `loadAgentForUser`, `requireUserID`, `parseUUIDOrBadRequest` in `server/internal/handler/handler.go`.
- Go tests are sentence-named: `TestListInboxProjectsCurrentIssueStatusAndPriority` (`server/internal/handler/inbox_test.go`), `TestEffectiveFailsSafeOnUnknownAndBrokenCatalog` (`server/internal/issuestatus/issuestatus_test.go`).

**Variables:**
- Wire/API JSON fields stay `snake_case` everywhere in TS (`workspace_id`, `created_at`, `issue_status`); the TS types mirror the wire shape (`packages/core/types/`). Local TS variables are `camelCase`.
- Workspace IDs in query keys and hooks are named `wsId` (`issueKeys.all(wsId)` in `packages/core/issues/queries.ts`).
- Persisted store names are `snake_case` with a `multica_` prefix: `name: "multica_create_mode"` (`packages/core/issues/stores/create-mode-store.ts`).
- Go sentinel errors are `ErrXxx` (exported) or `errXxx` (package-private): `ErrWakeupInput` in `server/internal/service/issue_wakeup.go`, `ErrUnknownStatus` in `server/internal/issuestatus/issuestatus.go`.

**Types:**
- TS types and interfaces are `PascalCase` with no `I` prefix and no `_t` suffix (`Issue`, `AgentRuntime`, `InboxItem`) — `apps/docs/content/docs/developers/conventions.mdx` § TypeScript.
- Prefer string-literal unions over `enum`: `export type CreateMode = "agent" | "manual";` (`packages/core/issues/stores/create-mode-store.ts`). Reserve `enum` for runtime-iterable cases.
- Zod schemas are `<Type>Schema`; their fallbacks are `EMPTY_<TYPE>` or `FALLBACK_<TYPE>` constants (`UserSchema` / `EMPTY_USER`, `AutopilotRunSchema` / `FALLBACK_AUTOPILOT_RUN` in `packages/core/api/schemas.ts`).
- Database: tables are singular `snake_case` (`user`, `workspace`, `agent_runtime`); foreign-key-like columns are `<table>_id`; booleans are `is_<state>` or `<state>_at` (timestamp form preferred) — `conventions.mdx` § Database.

**Routes and product names:**
- Pre-workspace routes are a single word or `/{noun}/{verb}` (`/login`, `/workspaces/new`), never hyphenated root names (`/new-workspace` collides with workspace slugs). New reserved slugs: update `server/internal/handler/reserved_slugs.json`, run `pnpm generate:reserved-slugs`, commit `packages/core/paths/reserved-slugs.ts`.
- Workspace-scoped routes live under `/{slug}/{section}`; shared code navigates through `useNavigation().push()` / `<AppLink>`, never framework link APIs.
- Never render `runtime.name` in UI copy; use `runtimeDisplayLabel(runtime)`, `runtimeDisplayName(runtime)`, `runtimeRowLabel(runtime, machine.title)` and group selectors with `buildRuntimeMachines` (`conventions.mdx` § Runtime display names).

## Code Style

**Formatting:**
- Go: `gofmt` + `go vet`, no exceptions (`conventions.mdx` § Go).
- TypeScript: no Prettier/Biome config is checked in. Match the surrounding file: 2-space indent, double quotes, semicolons, trailing commas in multi-line literals (see `packages/core/labels/mutations.ts`, `packages/views/settings/components/telegram-tab.tsx`).
- Files consumed by Next/React client code start with `"use client";` (`packages/core/issues/stores/create-mode-store.ts`, `packages/views/settings/components/telegram-tab.tsx`).
- Numeric literals use separators: `staleTime: 30_000` (`packages/core/issues/queries.ts`), `Limit: 1_000_000` (`server/internal/handler/handler_test.go`).

**TypeScript compiler (`packages/tsconfig/base.json`):**
- `strict`, `noUnusedLocals`, `noUnusedParameters`, `noImplicitReturns`, `noUncheckedIndexedAccess`, `isolatedModules`, `moduleResolution: "bundler"`. Prefix intentionally unused destructured params with `_` (`resource_type: _resourceType` in `packages/core/labels/mutations.ts`).

**Linting (ESLint 9 flat config, shared in `packages/eslint-config/`):**
- `packages/eslint-config/base.js`: `@eslint/js` recommended + `typescript-eslint` recommended; `@typescript-eslint/no-unused-vars` off (tsc enforces it); `@typescript-eslint/no-explicit-any` off; `import-x/no-extraneous-dependencies` = error (imports must be declared in the workspace's own `package.json`; devDependencies allowed only in tests, configs, `scripts/**`, `src/main/**`, `src/preload/**`).
- `packages/eslint-config/react.js`: `eslint-plugin-react` recommended + `jsx-runtime`; `react-hooks` `recommended-latest` applied to `.ts` too, so hooks in plain modules get `exhaustive-deps`.
- `packages/views/eslint.config.mjs`: `i18next/no-literal-string` (`mode: "jsx-text-only"`) on `*.tsx`, plus `no-restricted-syntax` selectors from `packages/views/eslint-i18n-guard.mjs` banning literal copy in `placeholder` / `title` / `aria-label` and in `toast(...)` arguments, including the fallback half of `err instanceof Error ? err.message : "…"`. Exempt technical values only with an inline `eslint-disable` plus a reason. The guard selectors are themselves tested in `packages/views/eslint-i18n-guard.test.ts`.
- `apps/desktop/eslint.config.mjs`: in `src/main/**` direct `shell.openExternal` / `webContents.downloadURL` are banned (use `openExternalSafely` / `downloadURLSafely` from `src/main/external-url.ts`); in `src/renderer/src/**` (outside `platform/`) `useNavigate`, `<Navigate>` and `router.navigate` are banned (MUL-4741 — navigate through the platform adapter).
- `apps/mobile/eslint.config.js`: `eslint-config-expo/flat` (`expo lint`), independent of the shared config.
- Repo-level guards: `knip.jsonc` (`pnpm knip`: unused files, dependencies, unlisted deps; ignores `apps/mobile`), `scripts/check-ui-radius-tokens.mjs` (`pnpm check:ui-radii`: no bare Tailwind radius utilities or arbitrary fixed radii outside an exact-file allowlist), `scripts/check-ui-wildcard-exports.mjs` (`pnpm check:ui-exports`: unreachable files in `packages/ui`).

## Import Organization

**Order (TS, observed in `packages/views/settings/components/telegram-tab.tsx`):**
1. React and third-party packages (`react`, `@tanstack/react-query`, `sonner`, `lucide-react`)
2. Workspace packages: `@multica/ui/...`, then `@multica/core/...`; type-only imports use `import type`
3. Relative imports within the package (`../../common/actor-avatar`, `../../i18n`)

The order is not lint-enforced; keep the file's existing grouping.

**Go (gofmt/goimports groups, `server/internal/handler/handler_test.go`):**
1. Standard library
2. One block with third-party and `github.com/multica-ai/multica/server/...` modules. The sqlc package is aliased: `db "github.com/multica-ai/multica/server/pkg/db/generated"`.

**Path Aliases and package entry points:**
- Shared packages export raw TypeScript via per-file `exports` maps, compiled by consumers: `@multica/core/api` → `packages/core/api/index.ts`, `@multica/core/issues/queries` → `packages/core/issues/queries.ts` (`packages/core/package.json`); `@multica/ui/components/ui/*` → `packages/ui/components/ui/*.tsx` (`packages/ui/package.json`); `@multica/views/...` (`packages/views/package.json`). Import the narrowest subpath that exists; add a new `exports` entry when a new module must be public.
- Inside `packages/core` and `packages/views`, use relative imports (`import { api } from "../api";` in `packages/core/labels/mutations.ts`).
- `@/*` alias: `apps/web/tsconfig.json` (`./*`), `apps/desktop/tsconfig.web.json` (`src/renderer/src/*`), `apps/mobile/tsconfig.json` (`./*`). `apps/web/vitest.config.ts` also maps `@core` → `apps/web/core`.
- Dependency direction is `views -> core + ui`; `core` and `ui` stay independent. `packages/core` must not import `react-dom`, `localStorage`, `process.env`, `next/*` or UI libraries (use `StorageAdapter` / `defaultStorage` from `packages/core/platform/storage`). `packages/ui` has no business logic and no `@multica/core` imports. `packages/views` has no store definitions and no `next/*` / `react-router-dom` (use `NavigationAdapter`, `useNavigation()`, `<AppLink>`).
- Mobile imports core types with `import type`; runtime imports from core are limited to pure utilities and platform-independent schemas (`apps/mobile/AGENTS.md` § Sharing and Dependencies).
- Shared external deps use `catalog:` from `pnpm-workspace.yaml`; mobile pins Expo/React Native versions in `apps/mobile/package.json` and adds native packages with `pnpm exec expo install <package>`.

## Error Handling

**Go handlers (`server/internal/handler/handler.go`):**
- Write errors as JSON via `writeError(w, status, msg)` → `{"error": msg}`; add a stable machine code with `writeErrorCode(w, status, code, msg)` so clients can translate; deliberate feature gates use `writeFeatureDisabled` (403, not 503). Return immediately after writing.
- Loader helpers write the error and return `ok=false`; callers must `return` right away:

```go
// server/internal/handler/activity.go
id := chi.URLParam(r, "id")
issue, ok := h.loadIssueForUser(w, r, id)
if !ok {
	return
}
```

- UUID handling (`AGENTS.md` § Backend UUID Rules): user-supplied UUIDs go through `parseUUIDOrBadRequest(w, s, "field_name")` (writes 400); trusted sqlc/fixture round-trips use the panicking `parseUUID(s)`; outside handlers use `util.ParseUUID(s)` and check the error. Never feed raw `chi.URLParam` / body strings into `parseUUID` on a write path.

```go
func parseUUIDOrBadRequest(w http.ResponseWriter, s, fieldName string) (pgtype.UUID, bool) {
	u, err := util.ParseUUID(s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+fieldName)
		return pgtype.UUID{}, false
	}
	return u, true
}
```

- Map `pgx.ErrNoRows` to 404 with `errors.Is`, everything else to 500 with a short lowercase message (`server/internal/handler/agent_builder.go`):

```go
if _, err := qtx.LockWorkspaceForChatSessionCreate(r.Context(), workspaceUUID); err != nil {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to lock workspace")
	return
}
```

- Transactions: `tx, err := h.TxStarter.Begin(ctx)`, `defer tx.Rollback(ctx)`, `qtx := h.Queries.WithTx(tx)` (`server/internal/handler/agent_builder.go`). No DB foreign keys or cascades: validate relationships and delete dependents in application code inside a transaction when atomicity matters.
- Workspace-scoped queries filter by `workspace_id`; membership gates access and `X-Workspace-ID` selects the workspace.

**Go services:**
- Declare sentinels with `errors.New` and wrap with context using `%w`: `fmt.Errorf("%w: issue is closed", ErrWakeupInput)` (`server/internal/service/issue_wakeup.go`). Handlers branch with `errors.Is(err, issuestatus.ErrUnknownStatus)` (`server/internal/handler/issue.go`).
- Fail safe rather than guess: `issuestatus.Effective` returns the key unchanged on DB error or unknown status (`server/internal/issuestatus/issuestatus_test.go` documents why).

**TypeScript API boundary (`AGENTS.md` § API Compatibility):**
- Every UI-consumed JSON response passes through a zod schema and `parseWithFallback` (`packages/core/api/schema.ts`), never an `as T` cast. Schemas default optional fields and use `.loose()`; the fallback must satisfy the success type:

```ts
// packages/core/api/client.ts
async getMe(): Promise<User> {
  const raw = await this.fetch<unknown>("/api/me");
  return parseWithFallback(raw, UserSchema, EMPTY_USER, {
    endpoint: "GET /api/me",
  });
}
```

- `parseWithFallback` logs a schema warning with the endpoint and returns the fallback (`packages/core/api/schema.ts`). Every `switch` over a server enum includes a `default` branch. Adding/changing an endpoint means updating its schema and malformed-response tests (`packages/core/api/schemas.test.ts`).
- HTTP failures throw `ApiError` (`packages/core/api/client.ts`) carrying `status`, `statusText` and the decoded `body`; branch on `body.code`, not the message text.
- UI surfaces errors with `sonner` toasts using translated fallbacks:

```tsx
// packages/views/settings/components/telegram-tab.tsx
} catch (e) {
  toast.error(
    e instanceof Error ? e.message : t(($) => $.telegram.toast_disconnect_failed),
  );
}
```

- Mobile uses `fetchValidated` (GET) / `fetchValidatedWith` (writes with consumed bodies) in `apps/mobile/data/api.ts` with schemas in `apps/mobile/data/schemas.ts` and `apps/mobile/lib/parse-response.ts`; raw `fetch` only for writes whose body is unused.

**State and mutations (`AGENTS.md` § State Rules):**
- TanStack Query owns server data; Zustand owns client state (filters, drafts, modals, tab layout). Persist only durable preferences/drafts/layout. Only auth/workspace stores call `api.*` directly.
- Workspace-scoped query keys include `wsId`; key factories return `as const` arrays built from a shared prefix (`issueKeys` in `packages/core/issues/queries.ts`).
- Optimistic field patches: cancel queries, snapshot, patch, roll back in `onError`, invalidate in `onSettled` (`useUpdateLabel` in `packages/core/labels/mutations.ts`). Create/delete/leave/confirm flows await the server before navigating; no optimistic deletes. Message sends keep visible pending/retry state.
- WebSocket events patch or invalidate Query caches (`packages/core/*/ws-updaters.ts`), never store server payloads in Zustand.
- Zustand selectors return stable references; use shallow comparison for allocated objects/arrays.

## Logging

**Go:** `log/slog` with key/value pairs, configured in `server/internal/logger/logger.go` (`logger.Init()` reads `LOG_LEVEL`, default `debug`; colors only on a TTY; `NewLogger(component)` / `NewWriterLoggerDefault` for daemon and migrate). Use the key `"error"` for errors (≈1040 occurrences vs ≈100 `"err"`). In handlers prepend request dimensions:

```go
// server/internal/handler/issue.go
slog.Warn("resolve issue status failed", append(logger.RequestAttrs(r), "error", err)...)
```

`slog.Warn` is the dominant level for degraded-but-handled paths; `slog.Error` for failures; `slog.Debug` for decision traces (`resolveActor` in `server/internal/handler/handler.go`).

**TypeScript:** namespaced console logger `createLogger(namespace)` from `packages/core/logger.ts` with `debug/info/warn/error`. Namespace is `<feature>.<area>`: `createLogger("chat.mut")` (`packages/core/chat/mutations.ts`), `createLogger("drafts.upload-coordinator")` (`packages/core/drafts/upload-coordinator.ts`). Use it for new logging in shared packages; only three non-test files in `packages/core` / `packages/views` call `console.*` directly (`packages/views/modals/create-issue.tsx`, `packages/views/common/task-transcript/transcript-button.tsx`, `packages/views/issues/surface/issue-surface.tsx`) — do not copy that. Mobile keeps its own request-ID logging in `apps/mobile/data/api.ts`.

## Comments

**When to Comment:**
- Code comments are English only; a Chinese comment is a bug (`conventions.mdx` § Comments in code).
- Comments explain *why*: the constraint, the past incident, the rejected alternative. Reference tickets/issues inline (`MUL-4195`, `#1661`, `MUL-4741`) — e.g. the `parseUUID` rationale in `server/internal/handler/handler.go` and the guard rationale in `packages/views/eslint-i18n-guard.mjs`.
- Tests document the regression they pin (`server/internal/issuestatus/issuestatus_test.go` lines above `TestResolveAcceptsBuiltInsWithoutACatalogRow`).
- Do not add internal compatibility shims, dual writes, fallback paths or legacy adapters unless requested (`AGENTS.md` § Change and Delivery Rules); API response compatibility is the exception.

**JSDoc/TSDoc and Go doc:**
- Exported Go identifiers carry doc comments starting with the identifier name (`// Insert writes one row into table, ...` in `server/internal/testutil/db.go`); packages have a `doc.go` when the contract needs prose (`server/internal/testutil/doc.go`).
- TS uses `/** ... */` on exported stores/functions and on non-obvious key-factory members (`/** PREFIX for invalidation — no sort. */` in `packages/core/issues/queries.ts`); line `//` comments elsewhere.

## Function Design

**Size:** No enforced limit. Handlers are long linear sequences of guard → parse → query → write; extract shared request plumbing into `handler.go` helpers and shared row-building into `server/internal/testutil`.

**Parameters:**
- Go handlers take `(w http.ResponseWriter, r *http.Request)`; helpers that may fail take `w` and return `(T, bool)`. Context is `r.Context()`.
- TS mutation hooks take no args and read `wsId` from `useWorkspaceId()`; hooks needing workspace context accept `wsId` unless guaranteed to run under its provider.
- Shared UI is injected via props/slots for platform-specific pieces; platform APIs stay in `apps/*` (`packages/views/platform`, `openExternal`).

**Return Values:**
- Go JSON responses go through `writeJSON` (buffers to set `Content-Length`). Nil vs empty slice is deliberate: `uuidsToStrings` returns `nil` so `omitempty` drops the field; `uuidStringsOrEmpty` returns `[]string{}` when empty is authoritative (`server/internal/handler/handler.go`).
- TS API methods return parsed domain types, never raw `unknown`.

## Module Design

**Exports:**
- `packages/core`: one folder per feature (`issues/`, `inbox/`, `labels/`, …) with `index.ts`, `queries.ts`, `mutations.ts`, `ws-updaters.ts`, `stores/`; each public module is listed in `packages/core/package.json` `exports`.
- `packages/ui`: shadcn/Base UI primitives in `packages/ui/components/ui/`, compositions in `packages/ui/components/common/`, tokens in `packages/ui/styles/tokens.css`. Add primitives with `pnpm ui:add <component>`; for `@reui/<name>` decline overwrite prompts and keep `REUI_LICENSE_KEY` in the environment only.
- `packages/views`: domain folders (`settings/components/`, `issues/components/`) with i18n namespaces in `packages/views/locales/<locale>/<ns>.json`.
- Mobile: generic primitives in `apps/mobile/components/ui/`, domain compositions in `apps/mobile/components/<domain>/`, data layer in `apps/mobile/data/` (`queries/`, `mutations/`, `realtime/`).

**Barrel Files:** Feature-level `index.ts` barrels exist in `packages/core/*/index.ts` and selected `packages/views/*/index.ts`, but deep subpath imports (`@multica/core/issues/queries`) are the norm. `packages/ui` exports per file via wildcards (no barrel).

## UI Rules (web/desktop)

**Button (`packages/ui/docs/button.md`):** import from `@multica/ui/components/ui/button`. Variants: `default` (main action), `outline` (secondary), `secondary`, `ghost`, `brand`, `brandSubtle`, `destructive`, `link`. Sizes: `xs`, `sm`, `default`, `lg`, `icon-xs`, `icon-sm`, `icon`, `icon-lg`. There is no `primary` variant and no `loading` prop — compose a spinner with `disabled` + `aria-busy`. Icon-only buttons need an accessible name. One dominant action per dialog/form; use links for navigation.

```tsx
<Button type="submit" variant="default">Save</Button>
<Button type="button" variant="outline" onClick={onCancel}>Cancel</Button>
<Button variant="ghost" size="icon-xs" aria-label="Add item"><Plus /></Button>
```

**Dialog (`packages/ui/docs/dialog.md`):** compose `Dialog`, `DialogTrigger`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter`, `DialogClose` (no `variant` prop). Use Popover for contextual options, AlertDialog for consequential confirmation. Close a submitting dialog only after success; keep input and show the error on failure. Never hand-roll `<div role="dialog">`. Motion tokens: `--dialog-enter-duration`, `--dialog-exit-duration`, `--dialog-enter-easing`, `--dialog-exit-easing`; respect `prefers-reduced-motion`.

**Typography tokens (`packages/ui/styles/tokens.css`):** product UI uses only the role-named `--text-*` scale, never Tailwind's default size ramp or arbitrary `text-[13px]` values: `text-micro` (11/15), `text-caption` (12/16), `text-label` (13/18), `text-body` (14/20), `text-body-lg` (15/22), `text-title-sm` (16/24), `text-title` (18/28), `text-title-lg` (20/28), `text-display-sm` (24/32), `text-display` (36/40). Landing-page display type is a separate ramp.

**Other UI rules (`AGENTS.md` § Web/Desktop UI Rules, § UI Copy):** prefer existing shadcn/Base UI primitives; use shared semantic tokens from `packages/ui/styles/`; radii only via `--radius-*` (enforced by `scripts/check-ui-radius-tokens.mjs`); selected states stay identifiable on hover; handle overflow/long text deliberately. Descriptions are optional and omitted by default — do not restate titles, labels or button actions; keep permissions, cost, destructive consequences and recovery visible. Desktop windows outside the dashboard shell mount `<DragStrip />` from `@multica/views/platform` and interactive controls in the top 48px need `WebkitAppRegion: "no-drag"`.

## i18n and Copy (`apps/docs/content/docs/developers/conventions.mdx`)

- All JSX text, `placeholder`/`title`/`aria-label` and toast copy in `packages/views` comes from `useT(namespace)` (`packages/views/i18n/use-t.ts`) with the selector API: `const { t } = useT("settings"); t(($) => $.telegram.toast_disconnected)`.
- Locales: `en`, `zh-Hans`, `ja`, `ko`, `fr` under `packages/views/locales/`; parity is tested by `packages/views/locales/parity.test.ts`.
- Key naming is three-level `feature.component.action` (`issues.toolbar.batch_update_success`); shared copy at namespace top level, web-only under `web`, desktop-only under `desktop`.
- Plurals use i18next `_one` / `_other` (Chinese fills only `_other`); interpolation uses `{{var}}`.
- Everyday nouns are translated (issue → 任务, workspace → 工作区); Multica-specific terms stay lowercase English (`skill`); role/status enums (`owner`, `in_progress`) render as lowercase English; brands and acronyms are never translated. Mixed Chinese/English has one space on each side of the English word; Chinese uses full-width punctuation and straight double quotes.
- Product noun for an agent execution record is "Run"; `task` remains only the API/DB identifier.

## Commits and Delivery

- Conventional, atomic commits: `feat(scope)`, `fix(scope)`, `refactor(scope)`, `docs`, `test(scope)`, `chore(scope)`; use the repository PR template (`conventions.mdx` § Commit messages, `AGENTS.md`).
- Regenerate sqlc with `make sqlc` after SQL changes (`server/sqlc.yaml` → `server/pkg/db/generated`, `emit_json_tags`, `emit_empty_slices`).
- Migrations: no foreign keys / cascades; every index uses `CREATE [UNIQUE] INDEX CONCURRENTLY`, one statement per migration file, with an idempotent down:

```sql
-- server/migrations/562_search_index_change_workspace_index.up.sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_search_index_change_workspace_xid
    ON search_index_change (workspace_id, change_xid, entity_type, entity_id);
```

- Conditionally skipped migrations are still recorded in `schema_migrations`; later DDL touching conditional objects uses `IF EXISTS` / `IF NOT EXISTS`.

---

*Convention analysis: 2026-10-12*
