## At a glance

| Group | Kind | Team | Builder |
|---|---|---|---|
| 1. New model | new model | Data Models | `madspec-model-builder` |
| 2. Types and schemas | model change | Data Models | `madspec-dto-builder` |
| 3. Action names | action names | Data Models | `madspec-model-builder` |

## 1. New model

- [x] 1.1 `server/migrations/732_agent_session.up.sql`, `.down.sql` -- create `agent_session` with the columns and `CHECK` of design.md decision 1, no foreign keys, and the comment saying why; the down migration drops it. When `main` has taken 732 to 734, take the next free numbers and rename every reference.
- [x] 1.2 `server/migrations/733_agent_session_daemon_pid_index.up.sql`, `.down.sql` -- `CREATE UNIQUE INDEX CONCURRENTLY agent_session_daemon_pid_unique ON agent_session (daemon_id, pid)`.
- [x] 1.3 `server/migrations/734_agent_session_agent_index.up.sql`, `.down.sql` -- `CREATE INDEX CONCURRENTLY agent_session_workspace_agent_idx ON agent_session (workspace_id, agent_id)`.
- [x] 1.4 `server/cmd/migrate/main.go` -- add 733 and 734 to the concurrent index cleanups -- `TestEveryConcurrentUpBuildHasCleanup` requires it.
- [x] 1.5 `server/pkg/db/queries/agent_session.sql` -- `UpsertAgentSession`, `UpdateAgentSession`, `DeleteAgentSession`, `ListAgentSessionsForDaemon`, `ListAgentSessions` as design.md decisions 1 and 2 set out, each filtered by `workspace_id`.
- [x] 1.6 `server/pkg/db/queries/workspace_delete.sql` -- delete the workspace's `agent_session` rows in the step that deletes agents. `server/internal/handler/workspace_delete_manifest_test.go` -- classify `agent_session`.
- [x] 1.7 `server/pkg/db/generated/` -- run `make sqlc` and commit the output.
- [x] 1.8 `server/internal/handler/agent_session_queries_test.go` -- DB tests on `testPool` calling `db.New(testPool)` directly, as `handler_test.go` sets up: a terminal session of an offline daemon is not listed and of an online one is; a `running` task is listed as a run with its `work_dir` and issue, a `completed` one is not; a run gets the repository and branch of the worktree at its folder, of its only worktree, and none with two worktrees elsewhere; another workspace's rows never appear; an upsert on the same `(daemon_id, pid)` replaces the row; update and delete with another `daemon_id` change nothing -- the list is the one place where these rules hold.

## 2. Types and schemas

- [x] 2.1 `packages/core/types/agent.ts`, `index.ts` -- `AgentSession`, `AgentSessionKind`, `AgentSessionStatus`, `AgentSessionsResponse` as design.md decision 4.
- [x] 2.2 `packages/core/api/schemas.ts` -- `AgentSessionSchema`, `AgentSessionsResponseSchema` with the defaults and fallbacks of decision 4.
- [x] 2.3 `packages/core/api/schemas.test.ts` -- malformed-response cases: missing `sessions`, an unknown `kind`, an unknown `status`, a run without `runtime`.

## 3. Action names

- [x] 3.1 `server/pkg/protocol/events.go` -- `EventAgentSessions = "agent:sessions"` with payload `{agent_id}`. `packages/core/types/events.ts` -- add it to the event union.

## Implementation Notes

- Persistence, cleanup, generated sqlc bindings, client types/schemas and event contracts are implemented in the files named above. The living model map is updated in `../../../../docs/system-design.md`.
- Upsert keeps the existing row ID and replaces its process/session fields only within the same workspace. A conflicting `(daemon_id, pid)` owned by another workspace returns no row; the backend must not treat it as a successful registration.
- The list selects runs first so sqlc infers nullable PID, runtime, issue and repository fields correctly. An absent issue has a null `issue_id` and an empty identifier.
- Verification uses an isolated PostgreSQL 17 container, never the production database. All six session query tests, two existing runtime-session tests, the workspace deletion manifest, three concurrent-index cleanup tests and migration checks pass. The changed Go packages pass `go vet`, and `go build ./...` passes.
- A direct SQL smoke exercises the generated list query: a registered terminal appears while its daemon is online, disappears while offline and reappears on reconnect. All three down migrations succeed and the table is absent afterward.
- The schema suite reports 235 passed and one failure in the untouched `QuestionSchema` test `drops events and links it cannot read and keeps the rest`: it expects a project link to be discarded, while the schema accepts it. All seven added session cases pass. Core typecheck passes; core lint has no errors and two existing warnings in `platform/core-provider.tsx`. Changed Go files are gofmt-clean.
- Crabbox SSH and sync work, but execution is unavailable: all three check slots are occupied, two by orphaned `agent.test` processes and the third by another long-running check. An isolated lease also cannot execute the prerequisite probe. Verification therefore runs locally, one bounded check at a time. Only this run's queued probes are cancelled; other jobs are untouched.
