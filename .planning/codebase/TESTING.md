---
last_mapped_commit: 80fb7a97dc7c0b6e79ccfd92174fd7e124541d09
last_mapped_at: 2026-10-12
---
# Testing Patterns

**Analysis Date:** 2026-10-12

Rules come from `AGENTS.md` § Testing and § Development and Verification, `apps/mobile/AGENTS.md` § Verification, and `server/internal/testutil/doc.go`. The fork has no remote CI: every check runs locally through `make check` / `scripts/check.sh`.

## Test Framework

**Runners:**
- TypeScript unit/component: Vitest `^4.1.0` (`catalog:` in `pnpm-workspace.yaml`) with `@testing-library/react` `^16.3.2`, `@testing-library/user-event` `^14.6.1`, `@testing-library/jest-dom` `^6.9.1`, `jsdom` `^29.0.1`.
- Go: standard `testing` package (Go `1.26.6`, `server/go.mod`). No testify, gomock or mockery — assertions are plain `if got != want { t.Errorf(...) }`.
- E2E: Playwright `@playwright/test` `^1.58.2` (root `package.json`), Chromium only.
- Mobile: Vitest in a Node environment only, plus a shell test for the iOS wrapper (`apps/mobile/scripts/ios-run.test.sh`).
- Desktop main-process extra: one `node:test` harness, `apps/desktop/src/main/window-state.node-test.mjs` (run by hand: `node --experimental-strip-types --test src/main/window-state.node-test.mjs`).
- Repo scripts: `scripts/*.test.sh` / `scripts/*.test.mjs` (e.g. `scripts/test-go.test.sh`, `scripts/check-ui-radius-tokens.test.mjs`).

**Vitest configs:**

| Workspace | Config | Environment | Setup | Include |
| --- | --- | --- | --- | --- |
| `packages/core` | `packages/core/vitest.config.ts` | node (default) | none | `**/*.test.{ts,tsx}` |
| `packages/views` | `packages/views/vitest.config.ts` | `jsdom` | `packages/views/test/setup.ts` | `**/*.test.{ts,tsx}` |
| `apps/web` | `apps/web/vitest.config.ts` | `jsdom` | `apps/web/test/setup.ts` | `**/*.test.{ts,tsx}`; aliases `@` and `@core` |
| `apps/desktop` | `apps/desktop/vitest.config.ts` | `jsdom` | `apps/desktop/test/setup.ts` | `src/**/*.test.{ts,tsx}`, `scripts/**/*.test.mjs`; alias `@` → `src/renderer/src` |
| `apps/mobile` | `apps/mobile/vitest.config.ts` | `node` | none | `lib/**/*.test.ts`, `data/**/*.test.ts` |
| `apps/docs`, `apps/ui-lab` | `apps/docs/vitest.config.ts`, `apps/ui-lab` package script | — | — | `apps/docs/lib/*.test.ts`, `apps/ui-lab/src/*.test.ts` |

All configs set `globals: true`; tests still import `describe/it/expect/vi` from `"vitest"` explicitly. `packages/ui` has no unit-test script; its `packages/ui/test/shimmer-text-fixture.tsx` and `shimmer-text-bundle.mjs` feed the E2E spec `e2e/shimmer-text.spec.ts`.

**Playwright configs:**
- `playwright.config.ts`: `testDir: "./e2e"`, ignores `**/perf/**`, `workers: 1`, `retries: 0`, `timeout: 60000`, `baseURL` from `PLAYWRIGHT_BASE_URL` → `FRONTEND_ORIGIN` → `http://localhost:3000`. Servers are never auto-started; they must already run.
- `playwright.perf.config.ts`: `e2e/perf/` only, one worker, no retries, 180s timeout, against a production build (see `e2e/perf/README.md`).
- `playwright.iframe.config.ts`: local-only, runs `e2e/iframe-scroll-bridge.spec.ts` on system Chrome via `page.setContent`; not part of `make check`.

**Assertion Library:**
- Vitest `expect` + jest-dom matchers (`toBeVisible`, `toHaveAttribute`) registered in each `test/setup.ts` via `import "@testing-library/jest-dom/vitest";`.
- Go: `t.Fatalf` / `t.Errorf` with `got = %v, want %v` messages.
- Playwright `expect` with locator assertions.

**Run Commands:**

```bash
pnpm typecheck                     # TS typecheck (excludes mobile)
pnpm test                          # turbo test --filter=!@multica/mobile (all Vitest suites)
pnpm --filter @multica/views test  # one workspace
pnpm --filter @multica/views exec vitest run settings/components/telegram-tab.test.tsx  # one file
make test                          # Go: ensure DB, migrate, scripts/test-go.sh --race
(cd server && go test ./internal/handler -run TestListInbox -count=1)   # one Go test (DB must be migrated)
pnpm exec playwright test          # E2E (backend + frontend must be running)
pnpm exec playwright test --config=playwright.perf.config.ts            # perf scenario
make check                         # typecheck → Vitest → Go → start services → Playwright
pnpm --filter @multica/mobile typecheck && pnpm --filter @multica/mobile lint && pnpm --filter @multica/mobile test
(cd server && MULTICA_RUN_REAL_AGENT_SMOKE=1 go test -tags=agentintegration ./pkg/agent -run '<test-name>' -count=1 -v)
```

`make check` / `scripts/check.sh` order: ensure Postgres → `pnpm typecheck` → `pnpm test` → `bash scripts/test-go.test.sh` → `go run ./cmd/migrate up` → `bash scripts/test-go.sh` → start backend/frontend if not already up → `pnpm exec playwright test`. Root frontend commands and `make check` do not cover mobile. Run checks inside a worktree with its own `.env.worktree` (separate DB/ports) so production data is never touched.

`scripts/test-go.sh` runs every package except `pkg/agent` first, then `./pkg/agent/...` throttled (`-p 2 -parallel 2`). Both go through `scripts/go-test-with-agent-cli-guard.sh`, which prepends sentinel binaries for every name in `scripts/agent-cli-command-names.txt` (`claude`, `codex`, `cursor-agent`, `omp`, …) to `PATH`; any invocation fails the run with `unexpected agent CLI invocation`.

## Test File Organization

**Location:** colocated beside the implementation, in the layer that owns the behavior (`AGENTS.md` § Testing):
- Shared logic → `packages/core/**` (`packages/core/inbox/ws-updaters.test.ts` next to `ws-updaters.ts`).
- Shared components → `packages/views/**` (`packages/views/settings/components/telegram-tab.test.tsx`).
- Platform wiring → `apps/web/**` (`apps/web/app/[workspaceSlug]/layout.test.tsx`), `apps/desktop/src/**` (`apps/desktop/src/main/daemon-profile.test.ts`).
- Mobile helpers/data layer → `apps/mobile/lib/**`, `apps/mobile/data/**` (`apps/mobile/data/mutations/projects.test.ts`).
- E2E → `e2e/*.spec.ts`; perf → `e2e/perf/`.
- Go → `<file>_test.go` in the same package (`server/internal/handler/inbox_test.go`), package-level `TestMain` in `handler_test.go` / `main_test.go`.

Never test shared behavior in an app suite. Give each behavior one canonical layer: helper tests own parsing/state matrices; component tests cover wiring, accessibility, happy paths and named regressions. Write the failing regression test before a behavioral fix.

**Naming:**
- TS: `<file>.test.ts` / `<file>.test.tsx`; E2E: `<feature>.spec.ts`.
- Go: `<file>_test.go`; real-agent smoke files `<agent>_integration_test.go` with `//go:build agentintegration`; regression files may carry a ticket (`server/internal/handler/admission_security_mul4525_test.go`).

**Scale (tracked files):** ≈473 test files in `packages/views`, ≈170 in `packages/core`, 59 in `apps/desktop`, 35 in `apps/web`, 35 in `apps/mobile`, 1160 Go `_test.go` files, 27 Playwright specs.

**Structure:**

```
packages/core/<feature>/{queries,mutations,ws-updaters}.test.ts   # cache/state matrices
packages/views/<domain>/components/<component>.test.tsx            # wiring + a11y + regressions
packages/views/test/{setup.ts,i18n.tsx}                            # shared jsdom patches, renderWithI18n
apps/{web,desktop}/test/setup.ts                                   # jsdom patches per app
server/internal/testutil/{db.go,http.go}                           # Go fixtures + handler harness
e2e/{fixtures.ts,helpers.ts,env.ts,*.spec.ts}                      # TestApiClient, login helpers
```

## Test Structure

**Vitest environment directive:** DOM-free `.test.ts` files in jsdom workspaces start with `// @vitest-environment node` (≈183 files). Do not add it when it would silently switch the code under test to an SSR path. The shared setup files bail out when there is no `window`:

```ts
// packages/views/test/setup.ts
// Everything below patches gaps in jsdom. Pure-logic suites opt out of jsdom
// with `// @vitest-environment node` and share this file, so there is no DOM to
// patch there — bail out rather than guard each stub.
if (typeof window !== "undefined") {
  // localStorage, matchMedia, ResizeObserver, elementFromPoint, scrollIntoView stubs
}
```

**Suite Organization (core, node environment):**

```ts
// packages/core/inbox/ws-updaters.test.ts
// @vitest-environment node
import { describe, it, expect, vi } from "vitest";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { onInboxIssueDeleted } from "./ws-updaters";
import { inboxKeys } from "./queries";

describe("onInboxIssueDeleted", () => {
  it("removes all inbox items referencing the deleted issue", () => {
    const qc = new QueryClient();
    qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), items);

    onInboxIssueDeleted(qc, wsId, "issue-a");

    const after = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
    expect(after?.map((i) => i.id)).toEqual(["i3", "i4"]);
  });
});
```

Cache-logic tests use a real `QueryClient` and the real key factories — never hardcoded key arrays.

**Component tests (views, jsdom):**

```tsx
// packages/views/settings/components/telegram-tab.test.tsx
it("opens the connect dialog and submits the pasted bot token", async () => {
  mockRegister.mockResolvedValue({ id: "i1", agent_id: "agent-1", status: "active" });
  renderUI(<TelegramAgentBindButton agentId="agent-1" agentName="Bot" />);
  await userEvent.click(screen.getByTestId("telegram-agent-connect"));
  const tokenInput = await screen.findByTestId("telegram-bot-token");
  expect(tokenInput).toHaveAttribute("type", "password");
  await userEvent.type(tokenInput, "123456789:AAtesttoken");
  await userEvent.click(screen.getByTestId("telegram-connect-submit"));
  await waitFor(() =>
    expect(mockRegister).toHaveBeenCalledWith("workspace-1", "agent-1", {
      bot_token: "123456789:AAtesttoken",
    }),
  );
});
```

Render through i18n with `renderWithI18n(ui, { locale })` from `packages/views/test/i18n.tsx`, which wraps the production `RESOURCES` so new namespaces never render keys-as-text. Older files inline a `TEST_RESOURCES` slice with `I18nProvider` (`telegram-tab.test.tsx`); prefer `renderWithI18n` for new tests.

**Go table-driven / subtest style:**

```go
// server/internal/testutil/http_test.go
func TestJSONRequestEncodesValuesAndPassesRawBodiesThrough(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body any
		want string
	}{
		{"map", map[string]any{"a": 1}, `{"a":1}`},
		{"string passthrough", `{"broken":`, `{"broken":`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := JSONRequest("POST", "/api/thing", tt.body)
			// ...
		})
	}
}
```

Named `t.Run` subtests for distinct failure modes (`TestEffectiveFailsSafeOnUnknownAndBrokenCatalog` in `server/internal/issuestatus/issuestatus_test.go`); `t.Parallel()` in pure tests (≈263 files). DB-backed handler tests share one workspace and generally do not call `t.Parallel()`.

**Patterns:**
- Setup: `beforeEach` resets hoisted refs and `vi.clearAllMocks()` (`resetFixtures` in `telegram-tab.test.tsx`); Go uses `TestMain` for suite fixtures and `t.Cleanup` (via `testutil`) for per-test rows.
- Teardown: E2E `test.afterEach(async () => { await api.cleanup(); })`; Go fixture rows delete themselves in reverse creation order.
- Assertions: user-visible behavior and accessible roles/test IDs in components; exact cache contents in core; status + decoded JSON in Go handlers.
- Comments above a test state the regression or product rule it pins.

## Mocking

**Framework:** Vitest `vi.mock` / `vi.hoisted` / `vi.fn` / `vi.mocked`. Go uses hand-written stubs and fakes (no mocking library). Playwright uses no mocks except the perf scenario's synthetic HTTP/WebSocket.

**Views — mock API at `@multica/core/api`, stores with callable `getState`:**

```tsx
// packages/views/settings/components/telegram-tab.test.tsx
const mockRegister = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: { registerTelegramBot: mockRegister },
}));

vi.mock("@multica/core/auth", () => {
  const useAuthStore = Object.assign(
    (sel?: (s: { user: { id: string } }) => unknown) =>
      sel ? sel({ user: { id: "user-1" } }) : { user: { id: "user-1" } },
    { getState: () => ({ user: { id: "user-1" } }) },
  );
  return { useAuthStore };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: mockToastError, message: vi.fn() },
}));

import { TelegramAgentBindButton } from "./telegram-tab"; // import after vi.mock
```

**Mobile — mock `@/data/api` so the native fetch chain never loads; keep the real mutation lifecycle:**

```ts
// apps/mobile/data/mutations/projects.test.ts
// @vitest-environment node
vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQueryClient: () => state.qc,
    useMutation: (options) => {
      const observer = new actual.MutationObserver(state.qc, options);
      return { mutateAsync: (variables: unknown) => observer.mutate(variables) };
    },
  };
});
vi.mock("@/data/api", () => ({
  api: { createProject: vi.fn(), listProjects: vi.fn() },
}));
```

**Go — interface stubs and fake binaries:**

```go
// server/cmd/server/health_test.go
type stubReadinessDB struct {
	pingErr    error
	pingCalls  atomic.Int32
}
func (s *stubReadinessDB) Ping(context.Context) error { s.pingCalls.Add(1); return s.pingErr }
```

```go
// server/pkg/agent/zeroclaw_test.go
func writeFakeZeroclawScript(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "zeroclaw")
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake zeroclaw: %v", err)
	}
	return bin
}
```

**What to Mock:**
- In views: `@multica/core/api`, Query hooks/options when testing component wiring, Zustand stores (callable + `getState`), `sonner`, platform adapters (`../../platform`), heavy child components.
- In mobile data tests: always `@/data/api`; workspace store selectors.
- In Go: external processes (agent CLIs via fake scripts in `t.TempDir()`), readiness/DB interfaces in pure unit tests, rate limiters swapped with `t.Cleanup` (`server/internal/handler/handler_test.go`).

**What NOT to Mock:**
- Views tests must not mock `next/*` or `react-router-dom` — shared code goes through the navigation adapter.
- Core cache tests use a real `QueryClient`; mobile mutation tests keep the real `MutationObserver`.
- DB-backed Go tests hit real PostgreSQL (migrated test DB), not a fake querier; pure packages (e.g. `server/internal/issuestatus`) use small in-file fakes (`newFakeQuerier`).
- Default Go tests never resolve or execute user-installed agent CLIs; pass a test-created fake or a missing executable path. New default agent command names go in `scripts/agent-cli-command-names.txt` so the guard catches them.

## Fixtures and Factories

**Go DB fixtures (`server/internal/testutil`):** New DB-backed tests build rows through a `Fixture` and drive handlers through `Call`; never open-code `INSERT ... RETURNING id` + `t.Cleanup` or a recorder/status/decode quartet (`server/internal/testutil/doc.go`). Helpers stay behavior-free — they never assert product rules. The package imports `testing` and must only be imported from `_test.go` files.

```go
// server/internal/handler/handler_test.go
var dbfx *testutil.Fixture   // set in TestMain: dbfx = testutil.New(pool, testWorkspaceID, testUserID)
```

```go
// server/internal/handler/inbox_test.go
func TestListInboxProjectsCurrentIssueStatusAndPriority(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Inbox filter projections", "inbox-filter-"+uuid.NewString())
	dbfx.Member(t, workspaceID, testUserID, "owner")
	issueID := dbfx.Issue(t, "Filtered issue", testutil.Cols{
		"workspace_id": workspaceID,
		"status":       "in_review",
		"priority":     "high",
	})
	dbfx.Insert(t, "inbox_item", testutil.Cols{ /* ... */ "issue_id": issueID })

	var items []InboxItemResponse
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox),
		inboxRequest(http.MethodGet, "/api/inbox", workspaceID)).
		Want(http.StatusOK).
		JSON(&items)
	// product assertions stay in the test
}
```

Fixture API (`server/internal/testutil/db.go`): typed builders `User`, `Workspace`, `Member`, `Issue` (auto-numbers per workspace), `Comment`, `Runtime`, `Agent`, `Task`, `Squad`, `SquadMember`, `Project`, `ChatSession` — each takes `over ...Cols` overrides; generic `Insert(t, table, Cols)`, `InsertNoID`, `Exec`, `Cleanup`, `QueryRow(...).Scan`, `Count`. `Raw("now() - interval '2 hours'")` inlines SQL expressions (literals and fixture-produced ids only). Cleanup runs in reverse creation order.

HTTP harness (`server/internal/testutil/http.go`): `JSONRequest(method, path, body)` (strings/`[]byte`/`io.Reader` pass through unencoded for malformed-payload tests), `WithHeaders(req, "X-User-ID", id, ...)`, `WithURLParams(req, "id", id)` for chi params, `Call(t, h, req)` → `.Want(status)` / `.WantOneOf(...)` (only when several statuses are genuinely correct) / `.JSON(&dest)` / `.Map()` / `.Text()`, and `Decode[T](t, h, req, want)`. Failure messages include the request line and response body. Adoption: `testutil.Call` appears in ≈82 test files; ≈189 handler test files still hold legacy `httptest.NewRecorder()` code — do not extend that pattern.

DB-backed suites connect via `DATABASE_URL` (default `postgres://multica:multica@localhost:5432/multica?sslmode=disable`) and `os.Exit(0)` with "Skipping tests" when the DB is unreachable (`TestMain` in `server/internal/handler/handler_test.go`). Run migrations first (`make test` does).

**TS test data:** in-file factories with `Partial<T>` overrides:

```ts
// packages/core/inbox/ws-updaters.test.ts
function makeItem(id: string, issueId: string | null, overrides: Partial<InboxItem> = {}): InboxItem {
  return { id, workspace_id: wsId, recipient_type: "member", /* ... */ ...overrides };
}
```

Mutable mock state lives in `vi.hoisted(() => ({ current: ... }))` refs reset in `beforeEach`.

**E2E (`e2e/fixtures.ts`, `e2e/helpers.ts`):** setup/teardown goes through `TestApiClient` (raw `fetch` + `pg`, zero build-time coupling to the web app). `login` sends a code, reads it from `verification_code` in Postgres, verifies, stores the JWT. Methods: `ensureWorkspace`, `markUserOnboarded`, `createProject`, `updateProject`, `deleteProject`, `createIssue`, `seedTableIssues`, `deleteIssue`, `createComment`, `addMemberByEmail`, `removeMember`, `deleteWorkspace`, `searchIssues`, `updateIssue`, `cleanup` (seeded issues via SQL, then created issues, then projects). `e2e/env.ts` loads `.env.worktree` before `.env`. Per-worker identities: `e2e-${worker}-${runId}@multica.ai`.

```ts
// e2e/issues.spec.ts
test.describe("Issues", () => {
  let api: TestApiClient;
  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page);    // injects multica_token into localStorage
  });
  test.afterEach(async () => {
    if (api) await api.cleanup();
  });
  test("issues page loads with board view", async ({ page }) => {
    await api.createIssue("E2E Board View " + Date.now());
    await reloadAppPage(page);
    await expect(page.locator("text=Backlog")).toBeVisible();
  });
});
```

**Location:**
- Go fixtures: `server/internal/testutil/`; suite constants in each package's `TestMain` file.
- TS: inline per test file; shared wrappers in `packages/views/test/`.
- E2E: `e2e/fixtures.ts`, `e2e/helpers.ts`; perf fixtures in `e2e/perf/fixtures/`.

## Coverage

**Requirements:** None enforced — no coverage config in any `vitest.config.ts` and no Go coverage gate.

**View Coverage:**

```bash
pnpm --filter @multica/core exec vitest run --coverage   # requires adding @vitest/coverage-v8 (not installed)
(cd server && go test -cover ./internal/...)
```

## Test Types

**Unit Tests:**
- `packages/core`: parsing, schemas, key factories, cache patchers, stores (`packages/core/api/schemas.test.ts`, `packages/core/issues/stores/*.test.ts`).
- Go pure packages with fakes (`server/internal/issuestatus/issuestatus_test.go`, `server/internal/testutil/http_test.go`).
- Mobile `lib/` helpers and `data/` updaters.

**Integration Tests:**
- Go DB-backed handler/service tests against real Postgres via `testutil` (`server/internal/handler/*_test.go`, `server/internal/service/main_test.go`).
- Views component tests with mocked API but real components and i18n.
- Real-agent smoke tests: only when explicitly authorized. Gated by `//go:build agentintegration` and `requireRealAgentSmoke(t)`:

```go
// server/pkg/agent/real_agent_smoke_integration_test.go
//go:build agentintegration
func requireRealAgentSmoke(t *testing.T) {
	t.Helper()
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("set MULTICA_RUN_REAL_AGENT_SMOKE=1 to allow real agent CLI and account access")
	}
	t.Log("REAL AGENT SMOKE TEST: this test may access an authenticated account and consume quota")
}
```

Each smoke test calls it first, then skips in `-short` mode and when the binary is not on `PATH` (`server/pkg/agent/cursor_integration_test.go`). Run one at a time: `cd server && MULTICA_RUN_REAL_AGENT_SMOKE=1 go test -tags=agentintegration ./pkg/agent -run '<test-name>' -count=1 -v`.

**E2E Tests:**
- Playwright against a running backend (8080 default) and frontend (3000 default), `e2e/*.spec.ts`. Perf scenario `e2e/perf/typing-under-live-runs.spec.ts` (MUL-7227) uses synthetic HTTP/WS against a production build.
- Mobile native rendering is not covered by automated tests: verify UI changes in the simulator/device (themes, keyboard, scrolling, navigation) and cross-client realtime by changing data from web; report which native checks were unavailable (`apps/mobile/AGENTS.md` § Verification).

## Common Patterns

**API compatibility / malformed-response tests** (required when adding or changing an endpoint):

```ts
// packages/core/api/schemas.test.ts
it("degrades malformed session metadata without dropping the Chat", () => {
  const parsed = ChatSessionSchema.parse({
    ...baseSession,
    channel_source: { channel_type: 42 },
    is_current_channel_route: "yes",
  });
  expect(parsed.channel_source).toBeUndefined();
  expect(parsed.id).toBe("chat-1");

  expect(parseWithFallback({ id: 42 }, ChatSessionSchema, EMPTY_CHAT_SESSION, {
    endpoint: "GET /api/chat/sessions/:id",
  })).toEqual(EMPTY_CHAT_SESSION);
});
```

Mobile mirrors this in `apps/mobile/data/*-schema.test.ts` and `apps/mobile/lib/*-schema.test.ts`.

**Async Testing:**

```tsx
await userEvent.click(screen.getByTestId("telegram-connect-submit"));
await waitFor(() => expect(mockToastError).toHaveBeenCalled());
expect(mockInvalidate).not.toHaveBeenCalled();
```

Timers: `vi.useFakeTimers()` (≈73 files, e.g. `packages/core/search-index/client.test.ts`). E2E waits on visible text/locators (`waitForPageText` in `e2e/helpers.ts`), never fixed sleeps.

**Error Testing:**
- TS: resolve the mocked API with a malformed body or reject it, then assert the error toast and that no success side effect fired (`"does not report success for a malformed install response"` in `telegram-tab.test.tsx`).
- Go: post malformed payloads via `testutil.JSONRequest` with a raw string body and assert the status with `.Want(http.StatusBadRequest)`; fail-safe paths use subtests per failure mode (`"database error"`, `"corrupt category"` in `server/internal/issuestatus/issuestatus_test.go`).

**i18n and lint guard tests:** `packages/views/locales/parity.test.ts` (locale key parity), `packages/views/eslint-i18n-guard.test.ts` (lints fixtures against the exact selectors the build enforces).

---

*Testing analysis: 2026-10-12*
