## At a glance

```mermaid
flowchart LR
  classDef work fill:#d9f2d9,stroke:#2e7d32,color:#172117
  B[Backend PR, Joeri<br/>groups 1-6]:::work --> F[Frontend PR, Thomas<br/>groups 7-10, stacked]:::work
```

| Part | Groups | Owner |
|---|---|---|
| Backend: data, logic, API, creator fix | 1-6 | Joeri |
| Frontend: web and desktop | 7-10 | Thomas |

Server checks: `cd server && go test ./...` with the test database, as AGENTS.md "Development and Verification" says. Frontend checks: `pnpm typecheck`, `pnpm test`.

## 1. Data

- [x] 1.1 `server/migrations/740_todo.up.sql` / `.down.sql`: the `todo` table as `design.md` "classDiagram" holds it, with CHECKs on `kind`, `actor_type` and `done_by`. Verify: `make migrate-up` and `make migrate-down` on the worktree database.
- [x] 1.2 `server/migrations/741_todo_open_index.up.sql` / `.down.sql`: unique `CONCURRENTLY` index on `(workspace_id, member_id, kind, source_key) WHERE done_at IS NULL`; `server/migrations/742_todo_list_index.up.sql` / `.down.sql`: `CONCURRENTLY` index on `(workspace_id, member_id, created_at DESC) WHERE done_at IS NULL`; both registered as AGENTS.md "Database" says. Verify: migrate up.
- [x] 1.3 `server/pkg/db/queries/todo.sql`: `OpenTodo`, `CloseTodos` (by member, kind and source key, or by kind and source key for all members), `CloseTodosOnIssue`, `ListOpenTodos`, `CountOpenTodosByWorkspace`, `GetTodoForMember`, `MarkTodoDone`; `make sqlc`. Verify: `go build ./...`.

## 2. Logic

- [x] 2.1 `server/internal/todo/`: kind constants, `Open` and `Close` that publish `todo:opened` / `todo:closed`; `server/pkg/protocol/events.go` gets both events; `server/cmd/server/listeners.go` sends them to `member_id`. Verify: a test that opening twice keeps one row and publishes once.
- [x] 2.2 `server/cmd/server/todo_listeners.go`: question open and close. Verify: tests for asked, answered (`any_one` and `each`), reassigned, withdrawn.
- [x] 2.3 Mention open and close, from comments and issue descriptions. Verify: tests for mentioned, own mention, member replies, mentioned again after Mark done.
- [x] 2.4 Assigned open and close. Verify: tests for assigned in Todo, in Backlog, status move inside and outside the set, reassigned.
- [ ] 2.5 Agent blocked and run failed open and close. Verify: tests for blocked in a run, blocked by a member, failed run, run starts again, issue leaves Blocked.
- [ ] 2.6 Review open and close, and its hiding behind an open question in list and count. Verify: tests for In Review, Waiting For QA, creator is assignee, approval question on the issue.
- [ ] 2.7 System problems: autopilot paused, quota reached, quick create failed open where today's notice is made, and close on resume, admitted run, successful retry. Verify: tests for each open and close.
- [ ] 2.8 Stop the moved notice types in `notification_listeners.go`, `service/task.go`, `autopilot_failure_monitor.go` and `service/autopilot_quota_notifications.go`; update the tests that expected them. Verify: `go test ./cmd/server/... ./internal/service/...`.

## 3. API

- [ ] 3.1 `server/internal/handler/todo.go` and routes in `server/cmd/server/router.go`: `GET /api/todos`, `GET /api/todos/summary`, `POST /api/todos/{id}/done` as `design.md` "API contract" says. Verify: handler tests for order, overdue, review hidden, summary per workspace, done by another member is 404.

## 4. Backfill

- [ ] 4.1 `server/migrations/743_todo_backfill.up.sql`: open to-dos from open questions, assigned issues, created issues In Review, and the unarchived notices of the moved types that are still open; then archive those notices. Verify: a migration test on seeded rows.

## 5. Creator

- [ ] 5.1 `CreateIssue` sets the run's `originator_user_id` as creator for `agent_create` and `quick_create`. Verify: tests for quick create, agent in a run, agent run without originator; the subscriber test that the member follows as creator.
- [ ] 5.2 `server/migrations/744_issue_creator_backfill.up.sql`. Verify: migration test; on production data 91 issues change.

## 6. Docs

- [ ] 6.1 `specs/docs/system-design.md`: the `todo` table and its flow. Verify: read-through.

## 7. Client data (Thomas)

- [ ] 7.1 `packages/core/types/todo.ts`, zod schemas with `parseWithFallback` in `packages/core/api/schemas.ts`, API client methods, `packages/core/todos/` queries (`todoKeys`), Mark done mutation with optimistic removal, `todo:*` handlers in `use-realtime-sync.ts`, desktop notification on `todo:opened`. Verify: unit tests.

## 8. Inbox (Thomas)

- [ ] 8.1 `packages/views/inbox/`: to-do rows, detail pane per kind, Mark done selecting the next row, empty state, as `designs/screens/todo-inbox.md` says. Verify: component tests.

## 9. Updates (Thomas)

- [ ] 9.1 Today's inbox list on its own Updates route: `paths.ts`, web route, desktop `routes.tsx`, tab presentation, sidebar item with a bell icon and no number. Verify: component tests and the page in the browser.

## 10. Badges (Thomas)

- [ ] 10.1 The sidebar Inbox number, the workspace switcher dot and the desktop dock badge read `GET /api/todos/summary`. Verify: `badge-convergence.test.tsx`, `app-sidebar.test.tsx`.

## Implementation Notes

- 1.1: Added `server/migrations/740_todo.up.sql` and `.down.sql` with the designed fields, bounded kind/actor/completion vocabularies, nullable system actor, and no foreign keys. `details` defaults to an empty object.
- 1.1 verification: `make migrate-up ENV_FILE=.env.worktree` passed against isolated database `multica_todo_inbox_660`. `make migrate-down` successfully reversed 740, then stopped at the historical forward-only migration 494. With Joeri's approval, restored the database with migrate-up and independently ran 740's down and up files through `psql -v ON_ERROR_STOP=1`; both passed. Historical migrations are unchanged.
- 1.2: Added the standalone concurrent index migrations 741 and 742, their concurrent drop rollbacks, and both invalid-index cleanup registrations in `server/cmd/migrate/main.go`. Migrate-up passed on the isolated worktree database; standalone down/down/up/up through psql also passed.
- 1.3: Added `server/pkg/db/queries/todo.sql`, generated `server/pkg/db/generated/todo.sql.go` and the `Todo` model in `generated/models.go`, plus `server/pkg/db/todo_test.go`. Reads project live issue/question fields, hide missing issues and reviews shadowed by a question, sort overdue then newest, and count only workspaces with current membership.
- 1.3 decisions: `member_id` uses the recipient's user ID, matching existing inbox recipients and `member.user_id`. A null member on close means all recipients; issue closes accept a set of kinds. Duplicate opens and repeated Mark done return `pgx.ErrNoRows`; callers can distinguish real transitions for event publication and retrieve an already-completed row with `GetTodoForMember`.
- 1.3 surprise: sqlc inferred a concatenated issue identifier (even a CASE returning NULL) as a nonnullable string. The issue-projection CTE and left join preserve its nullable `pgtype.Text` type, including system to-dos without issues. Fixed this and an incomplete question test fixture before the final passing run.
- 1.3 verification: `make sqlc`, `go -C server build ./...`, and `TEST_DATABASE_URL`-configured `go -C server test ./pkg/db -count=1 -v` passed (four tests). Tests cover vocabulary constraints, duplicate opens, completion/reopening and ownership, member/all-member/issue close scopes, workspace isolation, overdue/newest order, live source fields, nullable system fields, review suppression/reappearance, and account-level membership filtering.
- 2.1: Added `server/internal/todo/todo.go` and its transition test, protocol constants, member-private routing in `listeners.go`, and its audience test in `todo_listeners_test.go`. `todo.Service` publishes only real inserts/closures and shares live-source response projection with the future API. The DB-backed duplicate-open/close test passed against the isolated worktree database (`go -C server test ./internal/todo -count=1 -v`). No design deviation; the audience test runs with the listener group below.
- 2.2: Added question transitions in `todo_listeners.go`, registration in `main.go`, and asked/any-one/each/reassigned/withdrawn tests in `todo_listeners_test.go`. Reassignment opens only the event's new recipient, not other answerers who may have marked done. Targeted listener tests passed, including private realtime delivery. Fixture errors (answerers have no `id`; events require explicit IDs) were corrected. Server integration tests read `DATABASE_URL`, not `TEST_DATABASE_URL`; an initial run used the default database and failed because it had no todo table. Subsequent listener runs explicitly selected the isolated worktree database.
- 2.3: Updated `todo_listeners.go` and its tests for comment and newly added description mentions, self-mention exclusion, member replies closing every mention on the issue, and later mentions after Mark done. Description mentions use the issue ID as source key and no comment ID, since no comment exists; comments use their comment ID. Existing member/all/squad mention expansion is preserved; agents and platform comments do not create member to-dos. `TestTodoMentionLifecycle` passed on the isolated database.
- 2.4: Updated `todo_listeners.go` and its tests for member assignment in the exact Todo/Back From QA/Blocked/In Review set. Relevant status or assignee transitions retire the old row before opening its replacement; unrelated edits do not revive Mark done. `TestTodoAssignedLifecycle` passed, covering Backlog exclusion, Todo, a move inside/outside the set, and reassignment. No design deviation.
