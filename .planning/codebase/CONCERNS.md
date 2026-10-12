---
last_mapped_commit: 80fb7a97dc7c0b6e79ccfd92174fd7e124541d09
last_mapped_at: 2026-10-12
---
# Codebase Concerns

**Analysis Date:** 2026-10-12

Scope: whole repository (`appboypov/multica` fork of `multica-ai/multica`, merged up to upstream tag `v0.6.1`, plus fork commits `db5540fd9`..`80fb7a97d`). Fork-specific rules live in `AGENTS.md` ("Fork" section); mobile rules in `apps/mobile/AGENTS.md`.

## Tech Debt

**Workspace leave navigates before the server confirms (known debt):**
- Issue: `navigateAwayFromCurrentWorkspace()` runs *before* `leaveWorkspace.mutateAsync(...)`. It clears the platform singleton with `setCurrentWorkspace(null, null)` and pushes a new route, so the realtime `member:removed` handler's "current === lost workspace" check fails and does not start a second full-page relocate. Delete already uses await-then-navigate; leave does not.
- Files: `packages/views/settings/components/workspace-tab.tsx` (lines ~100-139 and the `onConfirm` handler at ~254-266), `packages/core/realtime/use-realtime-sync.ts` (handler), `AGENTS.md` ("Desktop Rules": "Existing workspace leave clears/navigates first ... known debt")
- Impact: If the leave request fails (network, 400 sole-owner race, 5xx), the user has already been moved out of the workspace and only sees a toast, even though they are still a member. This also breaks the repo rule that destructive flows await the server before navigating.
- Fix approach: Add a self-initiated marker for `member:removed` (same shape as the pending-delete registry the `workspace:deleted` handler uses), then switch leave to `await mutateAsync` → `navigateAwayFromCurrentWorkspace()` inside the try block, the same order delete uses. Add a regression test in `packages/views/settings/components/` that fails the mutation and asserts no navigation happens.

**~111 API client methods return unvalidated JSON:**
- Issue: `AGENTS.md` ("API Compatibility") requires UI-consumed JSON to pass through zod plus `parseWithFallback`. `packages/core/api/client.ts` validates 172 calls (`this.fetch<unknown>` → `parseWithFallback`), but 111 methods `return this.fetch(...)` directly with a declared `Promise<T>` type. Examples: `updateIssue`, `moveIssue`, `updateComment`, `resolveComment`, `addReaction`, `getAgent`, `createAgent`, `verifyCode`, `googleLogin`, `getAssigneeFrequency`. Runtime-profile methods use `this.fetch<RuntimeProfile>` casts (lines ~2384-2428).
- Files: `packages/core/api/client.ts`, `packages/core/api/schemas.ts`
- Impact: An installed desktop client that talks to a newer backend can crash or mis-render if a field is renamed, becomes nullable, or gets a new enum value, and no fallback absorbs it. Mutation responses are written straight into TanStack Query caches.
- Fix approach: Migrate methods one domain at a time (issues → comments → agents → auth) to `const raw = await this.fetch<unknown>(...)` plus a schema in `schemas.ts` and `parseWithFallback`. Add malformed-response cases to `packages/core/api/schemas.test.ts` / `client.test.ts` for each one.

**Hand-synced previewable-file lists (Go ↔ TS):**
- Issue: The allowlists of text-previewable extensions, content types, and basenames are kept in sync by hand in two languages (`TODO(follow-up)` in both places).
- Files: `server/internal/handler/file.go` (`isTextPreviewable`, comment at ~1344-1356), `packages/views/editor/utils/preview.ts` (~108)
- Impact: When the lists drift, the Eye button shows but the proxy returns 415, or the server accepts a type the client renders unhighlighted.
- Fix approach: Use the reserved-slugs pattern: a JSON source of truth in `server/internal/handler/`, a generator script next to `scripts/generate-reserved-slugs.mjs`, and a generated TS module. Add a test that fails when the generated output is stale.

**Workspace slug → UUID lookup on every request:**
- Issue: `resolveWorkspaceUUID` and `ResolveWorkspaceIDFromRequest` call `GetWorkspaceBySlug` on every slug-addressed request (`TODO: cache slug→UUID lookup`).
- Files: `server/internal/middleware/workspace.go` (lines ~86-140)
- Impact: An extra Postgres round-trip on every web/desktop API call. `UpdateWorkspace` in `server/pkg/db/queries/workspace.sql` cannot change `slug`, so caching is safe.
- Fix approach: Add a small TTL cache (in-process LRU, or Redis when `REDIS_URL` is set) keyed by slug. Invalidate it on workspace delete.

**Deprecated pre-v3 onboarding endpoints:**
- Issue: The file is a frozen copy of old onboarding logic, kept for desktop builds older than v3 ("DO NOT add features here").
- Files: `server/internal/handler/onboarding_shim.go` (623 lines), its router entries in `server/cmd/server/router.go`
- Impact: Dead weight and extra merge surface. Removal is gated on `X-Client-Version` telemetry, which a single-user fork does not have.
- Fix approach: This fork only runs its own desktop build (`make desktop-app`), so the shim can be deleted together with its router entries and tests. Upstream will keep merging changes into it, so delete it in a dedicated fork commit and expect it back as a conflict on later syncs.

**Kiro dual-field prompt payload:**
- Issue: `session/prompt` sends both `content` and `prompt` because Kiro's docs and CLI 2.1.1 disagree on the field name.
- Files: `server/pkg/agent/kiro.go` (~368-377)
- Impact: Each prompt is sent twice in the payload. If Kiro ever rejects unknown fields, the integration breaks.
- Fix approach: Pin the Kiro CLI version the daemon supports and drop the field that version does not use.

**Issue-table grouping limited to built-in statuses:**
- Issue: `validIssueStatuses` is still the 7 canonical keys for table grouping and filtering, so custom statuses cannot be a group or filter value ("KNOWN LIMITATION").
- Files: `server/internal/handler/issue.go` (~126-138), `server/internal/handler/issue_table_group.go`, `server/internal/handler/issue_table_facets.go`
- Impact: Workspaces that use custom statuses (see `docs/issue-status-lifecycle-rollout.md`) cannot group or filter the table by them.
- Fix approach: Build group descriptors and compound cell keys from the workspace status catalog (`validateIssueStatusKey` path), not `issuestatus.Canonical()`.

**Desktop updater code kept alive in the fork:**
- Issue: The fork replaced every check with `LOCAL_RELEASE_UPDATE_MESSAGE`, but `electron-updater` is still imported and configured: `autoDownload = true`, channel pinning, `update-*` listeners, and the `updater:download` / `updater:install` IPC handlers that still call `autoUpdater.downloadUpdate()` / `quitAndInstall()`. Hourly timers still fire and log a thrown error each time. The publish target in `apps/desktop/electron-builder.yml` (~124-127) still points at `multica-ai/multica`.
- Files: `apps/desktop/src/main/updater.ts`, `apps/desktop/src/renderer/src/components/updates-settings-tab.tsx`, `apps/desktop/src/preload/index.ts`, `apps/desktop/electron-builder.yml`, `apps/desktop/package.json` (`electron-updater` dependency)
- Impact: The settings tab still shows an "automatic updates" toggle that does nothing. The console gets an error every hour. A stale renderer that invokes `updater:download` would reach the upstream feed.
- Fix approach: Make `updater:download` / `updater:install` reject with the local-release message as well, stop scheduling timers, and hide the toggle in `updates-settings-tab.tsx`. Keep the diff small, because `updater.ts` changes upstream in every release.

**The local-release message is duplicated:**
- Issue: The same message string is defined separately in Go and TS.
- Files: `server/internal/cli/update.go` (`LocalReleaseUpdateMessage`), `apps/desktop/src/main/updater.ts` (`LOCAL_RELEASE_UPDATE_MESSAGE`)
- Impact: The wording drifts the next time either copy is edited.
- Fix approach: Accept the two copies (they live in different runtimes) and add a test on each side that asserts the exact text. Or have desktop read the message from `multica version --json`, if the CLI exposes it.

**Duplicated `.env` bootstrap blocks in the Makefile:**
- Issue: `selfhost` and `selfhost-build` each contain an identical 18-line block that generates `.env` with `sed` and has separate Darwin and GNU branches.
- Files: `Makefile` (lines ~87-107 and ~120-140)
- Impact: A fix to one copy can miss the other, for example when a new secret key is added.
- Fix approach: Move the block into one script under `scripts/` (alongside `scripts/local-env.sh`) and call it from both targets.

## Known Bugs

**The desktop app can still install an upstream CLI binary:**
- Symptoms: When the bundled `multica` binary is missing or unusable, `ensureManagedCli` downloads the CLI and `checksums.txt` from `https://github.com/multica-ai/multica/releases/latest/download` into userData.
- Files: `apps/desktop/src/main/cli-bootstrap.ts` (`GITHUB_LATEST_BASE`, line 17-18; `ensureManagedCli` ~151), caller `apps/desktop/src/main/daemon-manager.ts:528`
- Trigger: A desktop build whose `resources/bin/multica` was not produced (for example a `bundle-cli.mjs` failure inside `scripts/package.mjs`), or a bundled binary that fails its health check.
- Impact: This breaks the fork rule "No build from upstream is ever installed" (`AGENTS.md`, Fork section). The daemon would then run upstream code against the fork's server.
- Workaround: Make sure `make desktop-app` succeeds with the bundled CLI. Fix: in the fork, make `ensureManagedCli` fail with `LOCAL_RELEASE_UPDATE_MESSAGE`, or copy from `server/bin/multica` instead of downloading.

**`make selfhost` pulls upstream images into the fork's production stack:**
- Symptoms: `make selfhost` runs `docker compose -f docker-compose.selfhost.yml pull` and starts `ghcr.io/multica-ai/multica-backend:latest` / `multica-web:latest` in the same Compose project and `pgdata` volume that `make local-release` (`selfhost-build`) uses.
- Files: `Makefile` (`selfhost` target, ~87-118), `docker-compose.selfhost.yml` (lines 56, 222), `docker-compose.selfhost.build.yml`
- Trigger: Running `make selfhost` by habit or because `CONTRIBUTING.md` / `SELF_HOSTING.md` say to.
- Impact: Upstream backend migrations run against the fork's production database, and fork-only behavior (realtime auth ack, worktree brief, completion rule) silently disappears.
- Workaround: Only use `make local-release` / `make selfhost-build`. Fix: in the fork, make `selfhost` fail with a pointer to `make local-release`, or alias it to `selfhost-build`.

**Runtime "update available" compares against upstream releases:**
- Symptoms: `packages/views/runtimes/components/update-section.tsx` fetches `https://api.github.com/repos/multica-ai/multica/releases/latest` to decide whether a runtime can be updated. The update action then fails with the local-release message from `server/internal/daemon/daemon.go` (`handleUpdate` ~5017, `runUpdate` ~5131).
- Files: `packages/views/runtimes/components/update-section.tsx`, `server/internal/daemon/daemon.go`, `server/internal/daemon/auto_update.go`
- Trigger: A daemon that reports a release-shaped version (`vX.Y.Z`). Source builds that report `git describe` strings are skipped by `parseReleaseVersion`.
- Workaround: None needed for source builds. A fork-local version check (or hiding the section) would remove the dead affordance.

**Swimlane status counts shift when a hidden status is shown again:**
- Symptoms: A parent whose children are all hidden counts as a card. When that status is shown again, the parent becomes a lane header and the column count drops by one.
- Files: `packages/views/issues/components/swimlane-view.tsx` (~934-961)
- Trigger: Parent grouping enabled, with every child of a parent in hidden statuses.
- Workaround: None. Tracked upstream as a follow-up.

**iOS Safari long-press does not open the issue context menu:**
- Symptoms: The delegated `contextmenu` event covers right-click and Android long-press, but not iOS Safari long-press.
- Files: `packages/views/issues/actions/issue-actions-context-menu.tsx` (~35-37)
- Trigger: Mobile web on iOS Safari.
- Workaround: Use the row's explicit actions menu.

**Mobile gaps:**
- Symptoms: Reacting to the issue body has no long-press affordance. The blockquote border style needs simulator verification, and the "see TODO below" it refers to is missing.
- Files: `apps/mobile/components/issue/issue-reaction-row.tsx:9`, `apps/mobile/lib/markdown/markdown-style.ts:256-272`
- Workaround: None.

## Security Considerations

**Agents run with permission bypass on the host, and the fork points them at real checkouts:**
- Risk: Agent backends start CLIs with approval or sandbox bypass flags (`--dangerously-skip-permissions`, `bypassPermissions`, `--yolo`, `--allow-all-tools`, `--trust-all-tools`). The fork's runtime brief also tells agents to create their own worktrees under `~/Worktrees` from the clone under `~/Repos`, following `~/.agents/skills/madspec-git/SKILL.md`, instead of the daemon-managed execenv checkout. Prompt injection through issue or comment text can therefore run arbitrary commands with the user's credentials against the user's real repositories.
- Files: `server/pkg/agent/claude.go`, `server/pkg/agent/codex.go`, `server/pkg/agent/copilot.go`, `server/pkg/agent/cursor.go`, `server/pkg/agent/kiro.go`, `server/pkg/agent/opencode.go`, `server/internal/daemon/execenv/runtime_config_sections.go` (`worktreeRule`, ~286-290), `server/internal/daemon/execenv/local_worktree.go` (`finalizeOnAgentBranch`)
- Current mitigation: The daemon runs only on the owner's machine. Task tokens are bound to one workspace (`server/internal/middleware/workspace.go`, task-token branch).
- Recommendations: Keep agent-visible workspaces single-member, or restrict who can comment on agent-assigned issues. Prefer the agent CLIs' sandbox modes where available. Document the trust boundary in `CLI_AND_DAEMON.md`.

**Server brief hard-codes one workstation's layout:**
- Risk: `worktreeRule` embeds `~/.agents/skills/madspec-git/SKILL.md`, `~/Repos` and Linear branch naming into every task brief the daemon builds. A runtime on another machine, or one without that skill, gets instructions it cannot follow.
- Files: `server/internal/daemon/execenv/runtime_config_sections.go`, `server/internal/handler/squad_briefing.go`, `server/internal/service/builtin_skills/multica-platform/references/*.md`
- Current mitigation: Fork-only usage.
- Recommendations: Move the rule into configuration (an env var or workspace setting read by the daemon) so the code stays close to upstream and the rule can change without a rebuild.

**JWT sessions are not revoked server-side:**
- Risk: `Logout` only calls `auth.ClearAuthCookies`. `IssueCliToken` mints a bearer JWT with the default `AUTH_TOKEN_TTL` (30 days, `server/internal/auth/cookie.go:119`). There is no `jti` or session-version check in `server/internal/middleware/auth.go`, so a leaked CLI or desktop token stays valid until it expires. PATs (`server/internal/auth/pat_cache.go`) can be revoked; JWTs cannot.
- Files: `server/internal/handler/auth.go` (`IssueCliToken` ~750, `Logout` ~776), `server/internal/auth/jwt.go`, `server/internal/middleware/auth.go`
- Current mitigation: Disabled users are rejected (`auth.ErrTemporarilyDisabledUser`). Rotating `JWT_SECRET` invalidates every token.
- Recommendations: Set a shorter `AUTH_TOKEN_TTL` in `.env` for self-hosting, or add a per-user `token_version` claim checked in the auth middleware.

**No global request-body limit:**
- Risk: 141 handlers call `json.NewDecoder(r.Body)` without `http.MaxBytesReader`, and neither the router nor any middleware sets a body cap. Only selected endpoints are bounded (`issue_wakeup.go`, `onboarding_shim.go`, `plugin_package.go`, `autopilot_webhook.go`, and `file.go` through `ParseMultipartForm(maxUploadSize)`).
- Files: `server/cmd/server/router.go`, `server/internal/handler/*.go`
- Current mitigation: Most routes require authentication (user, PAT, task token, or daemon token).
- Recommendations: Add one middleware in `server/internal/middleware/` that caps JSON bodies (for example 1-4 MB) and lets upload routes opt out.

**Desktop renderer runs with `webSecurity: false`:**
- Risk: Same-origin protections are disabled in every renderer window, and `plugins: true` is enabled for the PDF viewer.
- Files: `apps/desktop/src/main/renderer-web-preferences.ts` (lines 31-58), test `apps/desktop/src/main/renderer-web-preferences.test.ts`, `apps/desktop/src/main/navigation-guard.ts`
- Current mitigation: `sandbox: true`, context isolation on, node integration off, a navigation guard, and `setWindowOpenHandler` → `openExternalSafely`.
- Recommendations: Move the renderer to a privileged custom protocol (the comment in the file describes the plan), then turn `webSecurity` back on and scope `plugins` to a dedicated PDF view.

**HTML injection sinks:**
- Risk: `dangerouslySetInnerHTML` renders highlighted code and math. It is safe only as long as highlight.js and KaTeX escape their input.
- Files: `packages/ui/markdown/CodeBlock.tsx` (171, 224), `packages/views/rich-content/rich-code-block.tsx:109`, `packages/views/common/task-transcript/detail-surfaces.tsx` (73, 266), `packages/views/editor/extensions/math.tsx` (27, 42), `packages/views/editor/code-block-static.tsx:78`, `packages/ui/components/ui/chart.tsx:95`
- Current mitigation: The HTML comes from highlighter or KaTeX output, not raw user HTML. Attachment previews use `sandbox="allow-scripts"` without `allow-same-origin` (`packages/views/attachments/attachment-preview-page.tsx:146`), and plugin guests are double-framed (`packages/views/plugins/surface-document.ts:105`, wrapper `packages/views/plugins/plugin-surface-frame.tsx:142`).
- Recommendations: Keep `katex` and highlighter versions current (`pnpm-workspace.yaml` catalog). Any new sink must take only highlighter output.

**`.env.make` copies of secrets on disk:**
- Risk: Every `make` invocation rewrites `$(ENV_FILE).make` (`.env.make` or `.env.worktree.make`) as a copy of the env file minus multi-line values.
- Files: `Makefile` (lines 7-14)
- Current mitigation: `umask 077`; ignored by `.gitignore:21` (`.env*`).
- Recommendations: Leave it ignored. Exclude it from backups and sync tools, the same way as `.env`.

**Dev verification code and default JWT secret:**
- Risk: `MULTICA_DEV_VERIFICATION_CODE` bypasses email codes, and `defaultJWTSecret` is a public string.
- Files: `server/internal/handler/auth.go` (~50, ~138-148), `server/internal/auth/jwt.go:14`, `server/cmd/server/main.go` (`jwtSecretBootError` ~302-343)
- Current mitigation: Both are refused when `APP_ENV=production`, and `docker-compose.selfhost.yml:75` requires `JWT_SECRET`.
- Recommendations: Make sure the fork's production `.env` sets `APP_ENV=production` (check existence only; never commit values).

## Performance Bottlenecks

**Active inbox list has no bound:**
- Problem: `ListInboxItems` returns every non-archived item for the recipient, with no `LIMIT` and no pagination.
- Files: `server/pkg/db/queries/inbox.sql` (`ListInboxItems`), `server/internal/handler/inbox.go` (`ListInbox` ~156-183)
- Cause: The archived view is capped at 200 groups (`ListArchivedInbox`), but the active list never got a cap. Agent-heavy workspaces generate many notifications.
- Improvement path: Add keyset pagination (`created_at, id`) to the query and handler, keeping the unpaginated response shape for old clients behind a default page size, and page it in `packages/core/inbox/queries.ts`.

**Most list queries are unbounded:**
- Problem: 166 of 224 `:many` `List*` queries in `server/pkg/db/queries/*.sql` have no `LIMIT`. Examples: `ListAgents` (`agent.sql`), `ListMembers` (`member.sql`), `ListProjects` (`project.sql`).
- Files: `server/pkg/db/queries/`
- Cause: These lists are workspace-scoped and assumed small.
- Improvement path: Add defensive limits like `ListCommentsForIssue` has (`server/pkg/db/queries/comment.sql:1-25`: newest-N window), starting with tables that grow per task or run.

**Realtime fan-out is in-memory unless Redis is configured:**
- Problem: Without `REDIS_URL`, the server uses the in-memory hub, which works on a single node only.
- Files: `server/cmd/server/main.go` (~447-594), `server/internal/realtime/hub.go`, `server/internal/realtime/redis_relay.go`, `docker-compose.selfhost.yml:70` (`REDIS_URL: ${REDIS_URL:-}`)
- Cause: The self-hosted compose file ships no Redis service.
- Improvement path: Fine for this fork's one-node stack. Add a Redis service only if the backend is ever scaled out.

**Heavy client modules:**
- Problem: Very large React modules re-render and are costly to parse. The code mentions earlier tab-switch freezes (`packages/views/issues/actions/issue-actions-context-menu.tsx` header).
- Files: `packages/views/issues/components/issue-detail.tsx` (3,972 lines), `packages/views/issues/components/table-view.tsx` (2,563), `packages/views/issues/components/issues-header.tsx` (2,482), `packages/views/issues/components/swimlane-view.tsx` (1,902)
- Improvement path: Use the existing perf harness (`e2e/perf/`, `playwright.perf.config.ts`, `scripts/perf-compare.mjs`) before and after splitting these components.

## Fragile Areas

**Fork-vs-upstream merge surfaces:**
- Files changed by the fork since `v0.6.1` (`git diff --stat v0.6.1 HEAD`):
  - Update paths: `server/cmd/multica/cmd_update.go`, `server/cmd/multica/cmd_update_test.go`, `server/internal/cli/update.go`, `server/internal/daemon/daemon.go` (`handleUpdate`, `runUpdate`), `apps/desktop/src/main/updater.ts`, `apps/desktop/src/main/updater.test.ts`
  - Agent brief and completion semantics: `server/internal/daemon/execenv/runtime_config_sections.go`, `server/internal/daemon/execenv/local_worktree.go`, `server/internal/handler/squad_briefing.go`, `server/internal/service/issue_wakeup_system.go`, `server/internal/service/builtin_skills/multica-platform/references/{issues,runtimes,squads}.md`, `apps/docs/content/docs/{issues,squads}.*.mdx`, plus tests `runtime_config_repo_ref_test.go`, `runtime_config_test.go`, `local_worktree_test.go`, `squad_parent_status_contract_test.go`
  - Realtime: `server/internal/realtime/hub.go` (`case "auth"` ack for the iOS cookie path, commit `24582a455`), `server/internal/realtime/hub_test.go`
  - Build/tooling: `Makefile`, `turbo.json` (removed `globalDependencies` on the CI workflow), `docker-compose.yml` (project name `multica-dev`), `AGENTS.md`, `apps/mobile/AGENTS.md`
  - Removed pipelines: `.github/workflows/*`, `.github/actions/`, `.github/ci-paths.json`, `.github/RELEASING.md`, `.goreleaser.yml`, `.vercelignore`, `scripts/ci-scope.mjs`, `scripts/ci-scope.test.mjs`
- Why fragile: `daemon.go` (10,623 lines) and `runtime_config_sections.go` change in almost every upstream release, so each sync can conflict there or silently undo fork wording. The fork's completion rule ("agents may set `done`") reverses upstream's contract ("`done` stays human"), so new upstream briefing text or tests can bring the old rule back.
- Safe modification: Keep fork changes in small, self-contained hunks, ideally constants (`worktreeRule`, `LocalReleaseUpdateMessage`). After `make upstream-sync`, grep for upstream phrases the fork removed (`multica repo checkout`, "`done` stays human") and run `go test ./internal/daemon/execenv/... ./internal/handler/ -run Squad`. Send generic fixes such as the auth ack upstream so the fork carries less.
- Test coverage: The fork's brief text is pinned by the execenv tests listed above. There is no test that the brief contains no upstream checkout instructions.

**`make upstream-sync` script:**
- Files: `scripts/upstream-sync.sh`
- Why fragile: (1) `FORK_REMOVED_PATHS` is a fixed list, so any new upstream pipeline file outside it (for example a new top-level CI or release config) gets merged in silently. (2) It verifies only `pnpm install --frozen-lockfile`, `pnpm typecheck`, `pnpm test`, and `go build ./...`. It runs no Go tests, no migrations, no lint, no mobile checks, no e2e, and no script tests. (3) `git rm -f` on paths in conflict resolves those conflicts by deletion before the conflict check, which is intended but hides upstream edits to those paths. (4) It depends on `gh release view` (an authenticated `gh` CLI) and on the remote named `upstream`.
- Safe modification: After a sync, run `make check` in a worktree (it runs Go tests against `multica-dev` Postgres and Playwright) and the mobile commands from `apps/mobile/AGENTS.md`. Extend `FORK_REMOVED_PATHS` whenever upstream adds pipeline files. Consider failing the sync when the merge adds any file under `.github/`.
- Test coverage: None. There is no `scripts/upstream-sync.test.sh`, unlike most scripts in `scripts/`.

**Giant Go files:**
- Files: `server/internal/daemon/daemon.go` (10,623), `server/internal/service/task.go` (8,095), `server/internal/handler/daemon.go` (6,117), `server/internal/handler/issue.go` (4,909), `server/pkg/agent/codex.go` (4,317), `server/internal/handler/comment.go` (4,204), `server/cmd/multica/cmd_issue.go` (3,594), `server/pkg/agent/hermes.go` (3,512), `server/internal/handler/agent.go` (3,053), `server/cmd/server/router.go` (2,672)
- Why fragile: Their state machines (claim and pause, update CAS, watchdogs, task lifecycle) are spread across thousands of lines, and they cause most upstream-sync conflicts.
- Safe modification: Change by narrow hunks and run the package's tests (`server/internal/daemon/daemon_test.go`, 6,580 lines). Do not reformat or move code inside them in the fork.

**Large shared TS modules:**
- Files: `packages/core/api/client.ts` (5,230), `packages/core/api/schemas.ts` (3,712), `apps/web/features/landing/i18n/{en,zh,ko,ja}.ts` (~3.5-4k each)
- Why fragile: Every endpoint goes through one class, and the landing copy must stay in sync across four locales.
- Safe modification: Add endpoints together with their schema and a malformed-response test. Edit landing copy in every locale file in the same change.

**Migration runner invariants:**
- Files: `server/migrations/` (589 up / 589 down; numbers 001-128 include 30 duplicate prefixes such as `128_autopilot_collaborator` / `128_comment_routing_escalation`), `server/cmd/migrate/main.go` (`preMigrationHooks`, `concurrentIndexCleanups`, `concurrentDownIndexCleanups`)
- Why fragile: Every `CREATE INDEX CONCURRENTLY` migration must have a matching entry in `concurrentIndexCleanups`. Conditionally skipped migrations are still recorded. 39 historical migrations (up to `113_lark_inbound_dedup_per_installation.up.sql`) still declare `REFERENCES`, and 66 index-creating migrations (latest `447_issue_properties_bigm_index_statistics.up.sql`) predate the concurrent-index rule. New migrations must follow the `AGENTS.md` rules: no FKs, one `CONCURRENTLY` index per file.
- Safe modification: Use the next unused number, write one statement per concurrent-index file, add the cleanup map entry (`TestEveryConcurrentUpBuildHasCleanup` enforces it), and run `make sqlc`.
- Test coverage: `server/cmd/migrate/*_test.go`. pg_bigm-specific tests skip when the extension is missing.

**Makefile env parsing:**
- Files: `Makefile` (lines 3-14), `scripts/local-env.sh`
- Why fragile: An `awk` filter drops values that start with `KEY="` and do not close the quote on the same line. Single-quoted multi-line values, or a `"` inside an unquoted value, would confuse it. Make then sees a different variable set than Docker Compose and the scripts, which read `ENV_FILE` directly.
- Safe modification: Keep multi-line values (PEM keys) double-quoted. Check with `make status` after editing `.env`.

## Scaling Limits

**Single-node, single-user fork deployment:**
- Current capacity: One Docker Compose stack (backend on 8080, web on 3000, Postgres `pgvector/pgvector:pg17`) from `docker-compose.selfhost.yml`, with no Redis.
- Limit: Realtime fan-out and the in-process caches only work on one backend replica. Agent concurrency is bounded by the machine running the daemon.
- Scaling path: Set `REDIS_URL` (relay in `server/internal/realtime/redis_relay.go`) before adding backend replicas. `deploy/helm/` exists, but this fork does not maintain it.

**Unpaginated workspace lists:**
- Current capacity: Fine at the current scale. `ListComments` keeps a defensive cap, and the comment in `comment.sql` puts p99 at ~30 and the max ever observed at ~1.1k comments per issue.
- Limit: Inbox, agents, members, and projects load everything in one response (see Performance).
- Scaling path: Keyset pagination as described above.

## Dependencies at Risk

**`github.com/robfig/cron/v3` v3.0.1:**
- Risk: Unmaintained upstream since 2020 [INFERENCE: based on the version pin; not checked online].
- Impact: Autopilot and maintenance scheduling depend on it (`server/internal/service/autopilot.go`, `docs/maintenance-jobs.md`).
- Migration plan: Pin and vendor-review the package, or replace it with a maintained fork when upstream does.

**`github.com/gorilla/websocket` v1.5.3:**
- Risk: Maintenance-mode library [INFERENCE], used by both hubs (`server/internal/realtime/hub.go`, `server/internal/daemonws/hub.go`; the latter uses `CheckOrigin: true` by design and relies on header-only daemon auth).
- Impact: Every realtime and daemon connection.
- Migration plan: Follow upstream's choice; do not swap it in the fork, because that would create a large merge surface.

**Mobile dependencies pinned off the shared catalog:**
- Risk: `apps/mobile/package.json` pins `react` 19.2.0, `react-native` 0.83.6, and `expo` ~55.0.23, while the root catalog uses `react` 19.2.3 (`pnpm-workspace.yaml`). `apps/docs` uses Next `^15.5.16` while `apps/web` uses Next `^16.3.4`.
- Impact: Shared code in `packages/core` imported by mobile (88 files import `@multica/core/*`, mostly `types`) must work on both React versions. Root checks skip mobile (`package.json` scripts filter `!@multica/mobile`).
- Migration plan: Upgrade with `pnpm exec expo install` per `apps/mobile/AGENTS.md`, and run the mobile checks after every upstream sync.

**Electron toolchain:**
- Risk: The fork builds an unsigned app (`CSC_IDENTITY_AUTO_DISCOVERY=false` in the `desktop-app` target), and `electron-builder` `^26.0.12` has a pinned override (`builder-util-runtime` 9.7.0 in root `package.json`). `electron` is `^39.2.6`.
- Impact: Gatekeeper prompts on install. The override can conflict with future electron-builder upgrades coming from upstream.
- Migration plan: Re-check the override after every upstream sync that touches `apps/desktop/package.json`.

**Turbo cache no longer keyed on the toolchain:**
- Risk: The fork removed `"globalDependencies": [".github/workflows/ci.yml"]` from `turbo.json`, and nothing replaced it (`.nvmrc` = `22`).
- Impact: After a Node upgrade, Turbo can replay cached typecheck and test results produced by the old runtime and hide incompatibilities.
- Migration plan: Add `.nvmrc` (and `package.json` `packageManager`) to `globalDependencies`.

## Missing Critical Features

**No remote or automated verification:**
- Problem: GitHub Actions is disabled and its workflows are deleted. Upstream CI ran `go vet`, race-mode Go tests (`scripts/test-go.sh --race`), agent-package race tests, `scripts/*.test.sh` (install, selfhost-config, worktree-db, dev-env), `knip`, `turbo lint`, mobile verify, desktop smoke, the openclaw config smoke, and UI performance. The fork's `make check` (`scripts/check.sh`) runs only `pnpm typecheck`, `pnpm test`, `scripts/test-go.test.sh`, migrations, `scripts/test-go.sh` (no race), and Playwright.
- Blocks: Detecting data races, lint and dead-code drift, mobile breakage, packaging regressions, and shell-script regressions.
- Fix: Add a `make check-full` target (or extend `scripts/check.sh` behind a flag) that also runs `pnpm lint`, `pnpm knip`, `go vet ./...`, `bash scripts/test-go.sh --race`, the `scripts/*.test.sh` suites, and the mobile commands. Run it as the gate in `scripts/upstream-sync.sh`.

**No fork-specific install path for the CLI:**
- Problem: `CLI_INSTALL.md`, `scripts/install.sh` and desktop `cli-bootstrap.ts` all install upstream release binaries. The fork's CLI exists only as `server/bin/multica` from `make build` (linked into `~/.local/bin/multica` per `AGENTS.md`).
- Blocks: Reproducible installs on a second machine without building from source.

## Test Coverage Gaps

**Auth and session endpoints:**
- What's not tested: `POST /auth/logout` (`Logout`) and `POST /api/cli-token` (`IssueCliToken`) have no Go handler test and no e2e test. No test file names those functions or routes.
- Files: `server/internal/handler/auth.go`, `server/cmd/server/router.go` (lines 1510, 1665)
- Risk: Regressions in cookie clearing or CLI token minting (for example disabled-user handling) go unnoticed.
- Priority: High

**Squad read and membership endpoints:**
- What's not tested: The `GetSquad`, `ListSquads`, `ListSquadMembers`, `RemoveSquadMember` and `UpdateSquadMemberRole` handlers. `/members/role` is only exercised from the CLI side against a stub server (`server/cmd/multica/cmd_squad_test.go:63`).
- Files: `server/internal/handler/squad.go` (1,243 lines), router lines ~2150-2151
- Risk: Permission or validation regressions in squad management.
- Priority: Medium

**DB-backed Go tests skip silently:**
- What's not tested: Many handler tests call `t.Skip` when the test DB or fixture is missing (for example `server/internal/handler/github_test.go` 13+ skips, `skill_import_archive_test.go`, `skill_import_duplicate_test.go`, `dingtalk_test.go`). Go tests contain 1,450 `t.Skip` calls in total.
- Files: `server/internal/handler/*_test.go`, `server/internal/testutil/`
- Risk: A run without Postgres reports green while skipping integration coverage. `scripts/upstream-sync.sh` does not run Go tests at all.
- Priority: High. Run Go tests through `make test` / `make check`, which ensure the DB and migrations first.

**Mobile app:**
- What's not tested: 35 test files cover 266 non-test source files in `apps/mobile/`. Root `pnpm test` / `typecheck` / `lint` exclude mobile, and so do `make check` and `make upstream-sync`. Mobile also re-implements domain logic that already exists in core (`apps/mobile/lib/issue-status.ts`, `filter-issues.ts`, `group-issues-by-status.ts`, `inbox-display.ts`), so parity can drift.
- Files: `apps/mobile/`
- Risk: Upstream syncs break the iOS app (which this fork actively uses; see realtime commit `24582a455`) without any signal.
- Priority: High. Run `pnpm --filter @multica/mobile typecheck|lint|test` after every sync.

**Fork-only flows:**
- What's not tested: `scripts/upstream-sync.sh`, the `local-release` / `desktop-app` Makefile targets, and the desktop CLI fallback in `apps/desktop/src/main/cli-bootstrap.ts`. No test asserts that the fork never reaches `github.com/multica-ai` release URLs.
- Files: `scripts/upstream-sync.sh`, `Makefile` (~384-397), `apps/desktop/src/main/cli-bootstrap.ts`, `packages/views/runtimes/components/update-section.tsx`
- Risk: Upstream code paths come back after a sync without anyone noticing.
- Priority: Medium. Add a grep-style guard test (in the `scripts/*.test.sh` style) for upstream release URLs outside the landing or download pages.

**Workspace leave failure path:**
- What's not tested: Leave failing after navigation has already happened.
- Files: `packages/views/settings/components/workspace-tab.tsx`
- Risk: The known-debt race described above has no regression guard.
- Priority: Medium

---

*Concerns audit: 2026-10-12*
