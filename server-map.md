# Server map: comments, issue changes, questions, runs

Worktree: `deliver-issue-changes-to-running-agents`. Facts below cite `path:line` unless marked [INFERENCE].

---

## 1. Comment creation → agent tasks

### HTTP → DB insert

| Step | Location |
|------|----------|
| Handler | `Handler.CreateComment` `server/internal/handler/comment.go:1703` |
| DB | `db.CreateComment` `comment.go:1888-1953`; SQL `server/pkg/db/queries/comment.sql:436-467` |
| Wakeup actor / transactional capture | `wakeup_actor.go:17-27`, `beginWakeupWrite` `wakeup_actor.go:30-42`, `wakeupWrite` `comment.go:1951-1953` |

After commit: `publish(protocol.EventCommentCreated, …)` `comment.go:1972-1979`; then `triggerTasksForComment` `comment.go:1991`.

### Trigger resolution

`triggerTasksForComment` `comment.go:2038-2054` → `computeCommentAgentTriggers` `comment.go:2701+` (mentions, assignee fallback `routeAssigneeFallback` `comment.go:3042-3068`, thread/conversation sources `comment.go:1527-1531`). `/note` opt-out `comment.go:2012-2027`. Steer to running turn: `task_supplement.go:140-178`.

### Enqueue

`enqueueSingleCommentTrigger` `comment.go:2636-2694` → `TaskService.EnqueueTaskForIssue` / `EnqueueTaskForMention` / `EnqueueTaskForSquadLeader` / `EnqueueTaskForThreadParent`.

### Coalesce / defer (comment waits for active run)

`resolveCommentTriggerEnqueue` `comment.go:2175-2286`:

1. Queued + same HEAD: `mergeCommentIntoPendingTask` `comment.go:2459-2521` → `MergeCommentIntoPendingTask` `server/pkg/db/queries/agent.sql:1887+`.
2. Active task: `hasActiveTaskForIssueAndAgent` `comment.go:2357-2368` → `DispatchDeferred` `comment.go:2377-2385`.
3. Dispatched/running: `registerPlannedCommentForActiveTask` `comment.go:2532-2551`.
4. On completion: `reconcileCommentsOnCompletion` `server/internal/handler/daemon.go:4510-4694` → `enqueueCommentAgentTriggers` `daemon.go:4674-4690`.

### Trigger evidence & comment ids on tasks

Kinds: `server/internal/attribution/attribution.go:104-125` (`EvidenceComment`, `EvidenceQuestionWake`, …). Rows: `trigger_comment_id`, `coalesced_comment_ids`, `trigger_evidence_kind`, `trigger_evidence_ref_id` via `CreateAgentTask` `server/pkg/db/queries/agent.sql:320-349`; merge updates evidence `comment.go:2492-2493`.

---

## 2. Issue changes, events, realtime

- **UpdateIssue** `issue.go:3831`: `issue:updated` `issue.go:4143-4167`; enqueue via `WillEnqueueRun` + `dispatchIssueRun` `issue.go:4195-4204`, `issue_trigger.go:84-93`. Description-only: no enqueue unless predicate fires. Attachments: `EventIssueAttachmentsChanged` `issue.go:4168-4175`.
- **Create**: `maybeEnqueueOnAssign` `server/internal/service/issue.go:881-921`.
- **Protocol types**: `server/pkg/protocol/events.go:4-48`.
- **Bus**: `server/internal/events/bus.go:8-73`; `handler.publish` `handler.go:758-766`.
- **Wakeup fact names**: `server/pkg/eventcontract/events.go:4-42` (no question events in `WakeupTypes` `events.go:35-42`).
- **Realtime**: `listeners.go:250-286` workspace fanout; `realtime/broadcaster.go:6-13` scopes.

---

## 3. Questions

- Handlers: `AnswerQuestion` `question.go:169-188`, `WakeQuestion` `question.go:278-306`.
- Answer path: `QuestionService.Answer` `question.go:597-634` (DB only; no task).
- Unseen: `CountUnseenQuestionAnswers` `question.sql:200-223`; `Wake` `question.go:885-892`.
- Claim hydration: `loadClaimQuestionAnswers` `daemon.go:3727-3799`.
- Wake task: `EnqueueTaskForQuestionWake` `task.go:1451-1505` (`question_wake` evidence).

---

## 4. Task lifecycle & daemon

- Table/status: `001_init.up.sql:127-140`; claim `ClaimAgentTask` `agent.sql:762-764`; `ClaimTaskForRuntime` `task.go:3672+`; `ClaimTaskByRuntime` `daemon.go:3802+`.
- WS: `daemonws/notifier.go:24-30`, `events.go:172-173`; `NotifyTaskEnqueued` `task.go:7205-7207`.
- Complete → reconcile comments `daemon.go:4448`; cancel → `EventTaskCancelled` `task.go:3038`.
- Mid-run comment steer: `EventDaemonTaskSupplementAvailable` `task_supplement.go:176-177`.

---

## 5. Reactions

- `comment_reaction` `026_comment_reactions.up.sql:1-10`; `AddReaction` `reaction.go:53-138`; `issue_reaction` `027_issue_reactions.up.sql:1-12`. No question reactions.

---

## 6. Issue wakeup

- CLI `cmd_issue_wakeup.go:21-80`; service run `issue_wakeup.go:1057-1106` (`issue_wakeup` evidence `issue_wakeup.go:1065`); scheduler `jobs_issue_wakeup.go:12-19`; vocabulary `eventcontract/events.go:35-42`.

---

## 7. Migrations & sqlc

`server/migrations/`, `sqlc.yaml:1-9`, `AGENTS.md:48,84-88`.
