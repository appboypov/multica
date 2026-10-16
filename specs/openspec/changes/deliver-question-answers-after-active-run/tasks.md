## At a glance

```mermaid
flowchart LR
  persistence[1. Backend: persistence] --> wake[2. Backend: wake service]
  wake --> lifecycle[3. Backend: lifecycle and recovery]
  lifecycle --> contract[4. Backend: API contract and docs]
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Team | Backend, lead Joeri | changed | `server/` |
| Contract | Existing wake response parser | changed | `packages/core/api/schemas.ts`, `packages/core/types/question.ts` |
| Plan | One bug-fix pull request, four dependent groups | new | `specs/openspec/changes/deliver-question-answers-after-active-run/tasks.md` |

All groups belong to Backend and ship together on the existing branch `deliver-question-answers-after-active-run`, based on `main`. Services and API workers implement the groups; no journey passes, separate model PR or frontend screen work is needed. Behavior is in the [agent-wake](specs/agent-wake/spec.md) and [live-run-input](specs/live-run-input/spec.md) deltas; technical decisions are in [design.md](design.md) and [ADR-0022](../../../../adr/0022-an-accepted-wake-waits-durably-for-the-agent.md).

## 1. Backend: persistence

- [ ] 1.1 `server/migrations/745_question_wake_pending.up.sql`, `server/migrations/745_question_wake_pending.down.sql` - create/drop `question_wake_pending` with the composite workspace/issue/agent primary key, first actor/source, answer anchor and creation time; no foreign keys or backfill - accepted intent must survive a restart without creating an agent task.
- [ ] 1.2 `server/pkg/db/queries/question_wake_pending.sql`, `server/pkg/db/queries/question_reach.sql` - add transactional read/upsert/delete and bounded recovery-list queries; retain the first request's attribution and earliest anchor; define wake blockers including active statuses and deferred retries, and identify current-wait failed tail tasks with no materialized child or later successor - coalescing and retry settlement must share one durable identity.
- [ ] 1.3 `server/pkg/db/queries/question.sql`, `server/pkg/db/queries/agent.sql` - use the design's single effective answer cutoff in answer count/list and ask-back queries, subtract handled ask-back receipts, and merge `question_wake_anchor` into task context without overwriting `head_sha` or other keys; retain retry-context copying and add parent locking/child lookup needed by retry settlement - a blind retry must not discard input or create duplicate deferred children.
- [ ] 1.4 `server/pkg/db/generated/`, `server/pkg/db/queries/supplement_test.go` - regenerate bindings with the repository's sqlc command and extend real-DB coverage for retained anchors, handled current events, later answer edits and already-claimed question-wake input - counts and claim selection must agree at the boundary.

## 2. Backend: wake service

- [ ] 2.1 `server/internal/service/question.go`, `server/internal/service/question_audience.go`, `server/internal/service/question_wake_pending.go` - make Wake accept and coalesce active-task requests, implement the shared issue-locked drain, use current scope and one effective cutoff, return an empty task ID only for an accepted wait, and write task/activity/event plus intent deletion atomically; preserve unchecked answers and member-question behavior - Answer with Wake, explicit Wake and ask-back must reach the same scheduling path.
- [ ] 2.2 `server/internal/service/task.go` - carry the retained anchor through existing task context, settle only retry-eligible failed tails of the current wait using `MaybeRetryFailedTask`, and serialize parent/child retry arbitration without replaying historical or superseded failures - deferred and post-commit retries retain their place ahead of the waiting wake.
- [ ] 2.3 `server/internal/service/question_test.go`, `server/internal/service/question_wake_pending_test.go` - replace active-task refusal expectations with acceptance and durable-wait assertions; cover one run for concurrent wakes, first-waker attribution, handled-input suppression, no-Wake input, ask-back parity, unavailable agents, transaction rollback and repeated drains - prove the consumer-visible transitions rather than function forwarding.

## 3. Backend: lifecycle and recovery

- [ ] 3.1 `server/internal/service/task.go`, `server/internal/handler/handler.go` - wire the narrow question-wake drainer in existing composition and call it from single/bulk finished notifications after retry/comment reconciliation, deduplicated per agent/issue pair - completion, failure, cancellation and orphan recovery must release waits without per-handler scheduling forks.
- [ ] 3.2 `server/internal/handler/daemon.go`, `server/internal/handler/daemon_question_wake_test.go` - hydrate waiting-wake tasks using their context anchor and fresh input, excluding their retry lineage from the cutoff; exercise answer-with-Wake and manual Wake through terminal APIs and the next claim - the run must receive the actual latest answer, not just a new queue row.
- [ ] 3.3 `server/cmd/server/question_wake_reconciler.go`, `server/cmd/server/main.go`, `server/internal/service/question_wake_pending_test.go` - add an initial and 30-second bounded sweep using `runPeriodicSweep` and the same drain; cover a new service instance reading persisted waits, a missed terminal callback, and a concurrent retry-settlement gap - restarting the server must require no further member action.
- [ ] 3.4 `server/internal/handler/fail_task_successor_test.go`, `server/internal/handler/daemon_question_wake_test.go` - cover queued/deferred retry blocking, failure through the retry chain with an unseen answer, an already-queued question-wake that claims all input, and input arriving after that claim - prevent both lost answers and duplicate follow-up runs at the claim/finish boundary.

## 4. Backend: API contract and docs

- [ ] 4.1 `packages/core/types/question.ts`, `packages/core/api/schemas.ts`, `packages/core/api/schemas.test.ts`, `server/internal/handler/question_test.go` - document `task_id: ""` as accepted but not queued, accept it in the existing parser, and cover HTTP 202 for a waiting Wake while preserving immediate-task and authorization behavior - do not convert accepted deferral into a malformed-response fallback or invent a fake task ID.
- [ ] 4.2 `CLI_AND_DAEMON.md`, `specs/docs/system-design.md` - update the existing Wake explanation and affected service/data relations for durable intent, terminal drain, recovery and task-context anchor - keep the shipped contract and living structure accurate without changing card visibility or daemon protocol.

## Implementation Notes

## Plan Change Log
