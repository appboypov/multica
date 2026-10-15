## At a glance

| Group | Kind | Team | Builder |
|---|---|---|---|
| 1. New model | new model | Data Models | `madspec-model-builder` |
| 2. Types and schemas | model change | Data Models | `madspec-dto-builder` |
| 3. Action names | action names | Data Models | `madspec-model-builder` |

## 1. New model

- [ ] 1.1 `server/migrations/732_agent_session.up.sql`, `.down.sql` -- create `agent_session` with the columns and `CHECK` of design.md decision 1, no foreign keys, and the comment saying why; the down migration drops it. When `main` has taken 732 to 734, take the next free numbers and rename every reference.
- [ ] 1.2 `server/migrations/733_agent_session_daemon_pid_index.up.sql`, `.down.sql` -- `CREATE UNIQUE INDEX CONCURRENTLY agent_session_daemon_pid_unique ON agent_session (daemon_id, pid)`.
- [ ] 1.3 `server/migrations/734_agent_session_agent_index.up.sql`, `.down.sql` -- `CREATE INDEX CONCURRENTLY agent_session_workspace_agent_idx ON agent_session (workspace_id, agent_id)`.
- [ ] 1.4 `server/cmd/migrate/main.go` -- add 733 and 734 to the concurrent index cleanups -- `TestEveryConcurrentUpBuildHasCleanup` requires it.
- [ ] 1.5 `server/pkg/db/queries/agent_session.sql` -- `UpsertAgentSession`, `UpdateAgentSession`, `DeleteAgentSession`, `ListAgentSessionsForDaemon`, `ListAgentSessions` as design.md decisions 1 and 2 set out, each filtered by `workspace_id`.
- [ ] 1.6 `server/pkg/db/queries/workspace_delete.sql` -- delete the workspace's `agent_session` rows in the step that deletes agents. `server/internal/handler/workspace_delete_manifest_test.go` -- classify `agent_session`.
- [ ] 1.7 `server/pkg/db/generated/` -- run `make sqlc` and commit the output.
- [ ] 1.8 `server/internal/handler/agent_session_queries_test.go` -- DB tests on `testPool` calling `db.New(testPool)` directly, as `handler_test.go` sets up: a terminal session of an offline daemon is not listed and of an online one is; a `running` task is listed as a run with its `work_dir` and issue, a `completed` one is not; a run gets the repository and branch of the worktree at its folder, of its only worktree, and none with two worktrees elsewhere; another workspace's rows never appear; an upsert on the same `(daemon_id, pid)` replaces the row; update and delete with another `daemon_id` change nothing -- the list is the one place where these rules hold.

## 2. Types and schemas

- [ ] 2.1 `packages/core/types/agent.ts`, `index.ts` -- `AgentSession`, `AgentSessionKind`, `AgentSessionStatus`, `AgentSessionsResponse` as design.md decision 4.
- [ ] 2.2 `packages/core/api/schemas.ts` -- `AgentSessionSchema`, `AgentSessionsResponseSchema` with the defaults and fallbacks of decision 4.
- [ ] 2.3 `packages/core/api/schemas.test.ts` -- malformed-response cases: missing `sessions`, an unknown `kind`, an unknown `status`, a run without `runtime`.

## 3. Action names

- [ ] 3.1 `server/pkg/protocol/events.go` -- `EventAgentSessions = "agent:sessions"` with payload `{agent_id}`. `packages/core/types/events.ts` -- add it to the event union.
