# Multica system design

Multica's system structure and data flow ([ADR-0017](../../adr/0017-a-to-do-is-a-row-the-server-opens-and-closes.md)).

The [living system prototype](../../prototypes/system/) holds the map. It runs from the world around Multica down to code paths, with its models and playable journeys. Its one source is [`system.json`](../../prototypes/system/system.json). This page keeps the notes the map does not show.

## Components

### Go server


Source-context object cleanup materializes one due intent before updating its lease
(`server/pkg/db/queries/source_context.sql`). Each attempt owns at most one intent;
the round limit bounds attempts, including failed object deletions.

### Daemon


Traecli completion (`server/pkg/agent/traecli.go`) closes stdin after the ACP
notification quiet window, then drains stdout and stderr before inspecting the
final result. A bounded drain and process reap stop a lingering process without
discarding final notifications emitted after stdin EOF.

### Release coordination

Owner changes cancel unstarted runs; started runs keep their result surface. Claim and start recheck the release's owner, current squad leader, existence and shipment under the repository boundary. Dispatched inputs stay immutable; later facts remain pending for a follow-up. Shipping uses the existing explicit endpoint and removes waits without cancelling its running publisher.

## Models

`routine_decision` is one receipt per rule and issue or release state. Its `id` is the SHA-256 of its input without the start period and threshold. A release decision's `issue_id` is the issue whose event made the release ready. `outcome` starts as `judging` and is `pending` while a Decider task runs. A proposal waits as `awaiting_brian` until Brian answers; every decision ends as `applied`, `stale` or `failed`. `reviews` appends Brian's agreed or corrected entries. `automatic_number` is unique per workspace and `question_id` is unique (migrations 781 and 782).

`task_supplement` holds one receipt per item and run, keyed `(item_id, task_id)`: `item_kind` is `comment`, `comment_edit`, `answer` or `issue_change`, and `comment_id` stays set for comments and their edits only. `RunInputService.Deliver` writes a `pending` receipt for every running run with `task-supplement-v1` on the item's issues, except the actor's own agent's run and runs the composer excluded; `content` snapshots the item's text at delivery, with a comment's file names and ids. `multica issue react` sets `seen_at` (👀) or `handled_at` (✅) once, on a `delivered` receipt of the reacting run only. At run end only `handled_at` counts: a comment or edit for the agent without it replays as a follow-up run, and an answer or ask-back without it stays unseen for Wake (ADR-0019, ADR-0022). Comment receipts and the `run_marks` of answers and activity entries carry these times to `packages/core/types/run-input.ts`.

### Client read contracts

`server/internal/handler/client_read.go` maps scoped query rows to the response models of [Client read contracts](../../prototypes/system/#/srv-client-read). A wait is an open issue assigned to the client's member in one of the client's organisations, of any type and with any labels. Wait detail returns `ClientWaitResponse` directly; lists include the visible total before pagination.

### Question contracts

`QuestionRow` is sqlc's `db.Question` in `server/pkg/db/generated/models.go`, generated from the migrations. The client `Question` and `QuestionAskResult` contracts live in `packages/core/types/question.ts`; `packages/core/api/schemas.ts` parses question responses. The server's question and ask DTOs live in `server/internal/service/question.go`; `POST /api/questions` creates one transactional ask and returns `{ask_id, questions}`. `server/internal/question/policy.go` derives status from current answerers under `any_one` or `each`. Question events feed `cmd/server/todo_listeners.go`, which opens and closes per-answerer to-dos; the overdue sweep still stamps `overdue_notified_at` but creates no inbox notice. The event payload is declared in `server/pkg/protocol/events.go` and `packages/core/types/events.ts`.

### To-do storage and flow

`server/pkg/db/queries/todo.sql` and its generated sqlc queries own persistence. The recipient `member_id` is a user's ID (`member.user_id`), not the membership row's ID. `OpenTodo` inserts a row only while that user is a member of the workspace. Optional source and actor IDs are logical references without foreign keys. A to-do's actor is a member or an agent; a system event opens it without one. `kind` distinguishes question, mention, assigned, agent-blocked, run-failed, autopilot-paused, quota-reached, quick-create-failed, review, check and instruction-change rows; `details` retains system-action data.

Agent-blocked and run-failed to-dos select the first non-null member ID from the run's `originator_user_id`, its `accountable_user_id`, the `delegated_from_task_id` source run's `originator_user_id`, then that source run's `accountable_user_id`. Source runs and recipients belong to the same workspace. The lifecycle listener and migration 754 use this order; unattributed work opens no to-do.

Open rows have `done_at IS NULL`. The partial unique index on `(workspace_id, member_id, kind, source_key)` makes repeated opens inert; closing changes only open rows and records `done_by` as `handled` or `member`. The second partial index supports workspace/member reads by descending `created_at`. `internal/todo.Service` runs each write and its visibility query in one transaction and publishes visible-list transitions after commit; a transactional producer publishes after its own commit. The visibility query compares the changed rows and affected reviews before and after the write, so questions hide and reveal reviews without completing them. Before it runs, the transaction locks each member and issue its question and review rows touch, so concurrent question writes see each other and a review hides and reveals once. Its `Response` projection is shared by the HTTP handler and `todo:opened`; `todo:closed` identifies the row and completion reason, or null when the row is hidden.

List and summary queries share the visible-open-row projection: live issue/question fields, no rows for missing issues, and no review row while an open question row exists for that member on the issue. Lists order overdue before newest. The list and Mark done routes are workspace-scoped; the summary is account-level across current memberships and omits zero-count workspaces. Mark done is member-private and idempotent. The summary supplies Inbox, workspace-switcher and desktop badge counts.

`packages/core/todos/` owns list and summary queries, optimistic completion with rollback, and cache updates from `todo:opened` and `todo:closed`. `packages/views/inbox/components/inbox-page.tsx` renders actionable rows and per-kind details; `updates-page.tsx` renders informational notices with read/unread and Archive. Web and desktop routes share these views. Notification banners select Inbox by `todo` or Updates by `issue`. Floating chat resolves a selected to-do's `issue_id` for its current issue. Unconfirmed quick-create outcomes use neutral copy and offer no context retry.

`inbox_item` remains the storage behind `/api/inbox` for Updates, separate from actionable to-dos. Question, mention, failed-run, paused-autopilot, quota and failed/unconfirmed quick-create notices no longer originate there. Migration 753 corrects agent-created issues to their run's originator member without changing actor/origin attribution, and makes that member's `issue_subscriber` row an active `creator` row unless they unsubscribed from the issue or a tree above it; migration 754 then seeds still-open to-dos and archives the moved notices. [ADR-0017](../../adr/0017-a-to-do-is-a-row-the-server-opens-and-closes.md) records the storage decision.

A question has exactly one asker (`question_one_asker`): an agent in a run, with `asker_agent_id` and `asked_in_task_id`, or a member outside a run, with `asker_user_id`. `Question` carries all three ids as strings; list rows carry `asker_agent_id` and `asker_user_id`. `asker_user_id` is `""` for an agent's question, and `asker_agent_id` and `asked_in_task_id` are `""` for a member's. A member asks through `POST /api/questions` or `multica question ask --on <issue>`; the first link is that issue and no run is linked, so the answers reach no run as input and the question offers no Wake (ADR-0021).

### Question Wake lifecycle

`QuestionService.Wake`, answers sent with `wake: true`, and ask-backs share the same scheduling path. `POST /api/questions/{id}/wake` returns HTTP 202 with the queued task ID for an immediate wake, or `task_id: ""` and the current `answer_count` when accepted but not queued. `QuestionWakeResult` in `packages/core/types/question.ts` and its parser in `packages/core/api/schemas.ts` accept both. Answers without Wake create no waiting intent or follow-up run; member questions wake no agent.

`question_wake_pending` is keyed by `(workspace_id, issue_id, agent_id)` and stores no answer text. Its upsert retains the first member/source attribution and the earliest `answer_anchor`. Queued, dispatched, running and waiting-local-directory tasks and deferred retry children block a wake for that agent and issue. The intent is durable state, not an extra queue task.

Workspace teardown removes `question_wake_pending` by `workspace_id` in `DeleteWorkspaceIssueRoots`, alongside the other question state. The deletion manifest classifies it as workspace-owned; other workspaces' waiting intents remain intact.

`handler.New` wires `QuestionWakeDrainer` into `TaskService`. Single and bulk finished notifications drain once per agent/issue pair after terminal comment follow-ups and retry decisions. `cmd/server/question_wake_reconciler.go` uses `runPeriodicSweep` at startup and every 30 seconds to drain bounded batches after a restart or a missed callback. Both paths first settle retry-eligible failed tails belonging to the current wait through `MaybeRetryFailedTask`; parent-row locking and the child lookup arbitrate concurrent retry decisions. Historical or superseded failures are not retried for a wake.

`question_wake_transition.go` brackets the issue and comment handlers' enclosing deletion/replacement operations with issue-scoped advisory transaction locks. Wake consumption takes a shared advisory lock before its row locks; terminal callbacks and recovery skip an active transition, while explicit Wake waits. The deferred finish releases the guard and drains waiting pairs after deletion, replacement or failure recovery. The guard uses a separate transaction so the enclosing writes can acquire issue rows. Connection loss releases it; durable intent remains recoverable.

The shared drain locks the issue, pending intent and source question in that order, rechecks blockers and availability, and selects unseen answers and ask-backs from the live question links and parent tree. An event is handled only through this agent's `answer` receipt with `handled_at` set. No unseen input means deleting the intent without a task. Otherwise the transaction creates one task through `EnqueueTaskForQuestionWake`, records the first waker's activity and `woken` event, and deletes the intent. Notifications are published only after commit. Collisions or database/retry-settlement errors retain the intent. An archived agent or one without a runtime retains the wait and unseen input; a deleted target or invalid source scope is retired without a task.

The retained input boundary travels from `question_wake_pending.answer_anchor` into the existing JSONB `agent_task_queue.context.question_wake_anchor`; merging it preserves other context keys such as `head_sha`, and retry tasks inherit context. For an accepted wait, the effective cutoff is the greater of that anchor and the latest claim of a subsequently started `question_wake`, excluding the task being hydrated and its retry ancestors. Ordinary retry claims do not advance it. Card counts, drain selection and `loadClaimQuestionAnswers` use the same calculation and handled-event checks; a changed answer has a fresh current event. The claim loads current answer text and ask-backs rather than an enqueue-time snapshot. Immediate wakes without a retained anchor use the latest-started-run cutoff.

The daemon's `Task.QuestionAnswers` contract, live notices, prompt rendering and card visibility rules remain unchanged. [ADR-0022](../../adr/0022-an-accepted-wake-waits-durably-for-the-agent.md) owns the durable-wait and retained-boundary decision; [the CLI and daemon guide](../../CLI_AND_DAEMON.md#question-wake) describes the runtime behavior.

### Checks and instruction notes

| Route | Caller |
|---|---|
| `POST /api/todos/checks` | `multica todo check` |
| `GET /api/todos/checks/{id}` | `multica todo get` |
| `POST /api/todos/{id}/answer` | `multica todo answer`; `ApiClient.answerCheckTodo` through `useAnswerCheckTodo` in `TodoCheckBody` and `TodoInstructionBody` |
| `POST /api/todos/checks/{id}/instruction-note` | `multica todo instruction-note` |
| `POST /api/todos/{id}/undo-result` | `multica todo undo-result` |
| `GET`, `PUT /api/todos/check-settings` | `multica todo settings` |
| `GET /api/projects/{id}/check-trust` | `multica todo trust`; `ApiClient.getProjectCheckTrust` through `useProjectCheckTrust` in `ProjectCheckRecord` |
| `PUT /api/projects/{id}/check-trust/{kind}` | `ApiClient.setCheckSampling` through `useSetCheckSampling` in `ProjectCheckRecord` |
| `GET /api/projects/{id}/check-trust/{kind}/history` | `ApiClient.getCheckHistory` through `useCheckHistory` in `ProjectCheckRecord` |

Check kinds are `data_model`, `data_flow`, `design`, `prototype`, `feature`, `improvement`, `agent_tool` and `release`. A check is a `todo` row of kind `check` whose `details` hold `project_id`, `check_kind`, the work link, the sample flags and the answer. An instruction note is kind `instruction_change` with `source_key` set to its check's id. `project_check_trust` holds one record per project and kind with its recipient, counts and sampling mode; `check_settings` holds the workspace's `sample_every`. The app parts ship in the shared inbox pull request; `ProjectCheckRecord` also renders on the project page.

An answer or Undo request reaches the opening agent in this order: a receipt on its running run with `task-supplement-v1`, the handoff note of its unclaimed queued or deferred run, or a new run with the message. A dispatched run's prompt is already built, so behind it the new run is deferred and `PromoteDueDeferredTasksForRuntime` queues it once the slot frees. Every check read inside a check transaction uses that transaction.

### Agent session contracts

`server/pkg/db/queries/agent_session.sql` owns the registered-session writes and the combined `ListAgentSessions` query. Its inputs are `agent_session`, `agent_runtime`, `agent_task_queue`, `worklane`, `issue_worktree`, `repository`, `agent`, `issue` and `workspace`; sqlc emits `AgentSession` and `ListAgentSessionsRow` in `server/pkg/db/generated/`. Lane runs follow `issue.worklane_id` to the workspace-scoped lane and repository, with branch derived from the workspace prefix and issue number (ADR-0014); non-lane runs follow `issue_worktree` to repository (ADR-0007). Workspace teardown removes registered sessions with the agents.

The client contract `AgentSession` and its `{sessions}` response live in `packages/core/types/agent.ts`, with boundary schemas in `packages/core/api/schemas.ts`. The `agent:sessions` event carries `{agent_id}` in `server/pkg/protocol/events.go` and `packages/core/types/events.ts`. `server/internal/handler/agent_session.go` serves the daemon's register, update, end and list routes under `/api/daemon/agent-sessions` and the member-facing `GET /api/agents/{id}/sessions`; `server/cmd/server/router.go` applies their daemon and member authentication. Writes enforce workspace and daemon ownership and publish `agent:sessions`.

`server/internal/daemon/health.go` accepts local `POST /sessions` with `workspace_id`, `agent_id`, `pid`, `cwd` and optional `herdr_pane_id`. `agent_sessions.go` registers the process through `client.go`, owns a global Herdr lifecycle connection and independent per-pane status observers for registered panes, reloads sessions per served workspace on startup, and reconciles process start time, repository and branch every 30 seconds. Its observed/persisted pane identity state feeds optional `AgentSessionUpdate.HerdrPaneID` writes through `client.go` to the handler and `UpdateAgentSession` query; see the [observer isolation and reconciliation design](../openspec/changes/archive/2026-10-16-agent-active-sessions/design.md#2-herdr-keeps-a-panes-session-current). Daemon shutdown stops observation without ending independently running sessions. Multica runs remain sourced from the task queue and never inherit the daemon's Herdr pane.

## Absent

- No remote CI: `.github/` holds only `ISSUE_TEMPLATE/` and `PULL_REQUEST_TEMPLATE.md`; verification is `make check` in `Makefile`.
- No release feed: `multica update` answers `LocalReleaseUpdateMessage` (`server/internal/cli/update.go`); releases come from `make upstream-sync` and `make local-release`.
- No Redis in the self-host stack (`docker-compose.selfhost.yml`): the realtime hub and runtime request stores run in memory on one node.
- No new foreign keys or cascades (`AGENTS.md`); tables since `server/migrations/173_remove_resource_label_foreign_keys.up.sql` relate by id columns only, while early tables such as those in `001_init.up.sql` keep theirs.
- No direct server access to a runtime machine: the server reaches it only through daemon heartbeat responses and `/api/daemon/ws`.
- No bundled agent CLIs: the daemon finds them on `PATH` or `MULTICA_<ID>_PATH` (`server/internal/daemon/agents_probe.go`, `server/pkg/agent/builtin_runtimes.go`).
