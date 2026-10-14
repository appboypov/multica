## At a glance

### Containers

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  member["Workspace member<br/>web or desktop"]:::changed
  server["Go server<br/>receipts for comments, answers, issue changes"]:::changed
  pg[("PostgreSQL<br/>task_supplement")]:::changed
  daemon["Daemon<br/>supplement loop"]
  omp["omp agent CLI<br/>--mode rpc --no-ui, steer"]:::changed
  cli["multica CLI<br/>issue react"]:::changed
  member -->|"comment, answer, issue change"| server
  server --> pg
  server -->|"WS daemon:task_supplement_available"| daemon
  daemon -->|"POST supplements/claim, ack"| server
  daemon -->|"steer frame on stdin"| omp
  omp -->|"runs"| cli
  cli -->|"POST /api/tasks/{id}/inputs/{item}/reactions"| server
  server -->|"WS run_input:reacted"| member
```

### Go server

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  comment["handler/comment.go<br/>steerCommentAgentTriggers"]:::changed
  qsvc["service/question.go<br/>Answer, EditAnswer, AskBack"]:::changed
  act["cmd/server/activity_listeners.go<br/>issue:updated rows"]:::changed
  runinput["service/run_input.go<br/>RunInputService"]:::new
  react["handler/run_input.go<br/>ReactToRunInput"]:::new
  supp["handler/task_supplement.go<br/>ClaimTaskSupplement, receipts"]:::changed
  reconcile["handler/daemon.go<br/>reconcileCommentsOnCompletion"]
  sql["pkg/db/queries<br/>supplement.sql, comment.sql, question.sql"]:::changed
  bus["internal/events Bus"]
  comment --> runinput
  qsvc --> runinput
  act --> runinput
  runinput --> sql
  runinput -->|"NotifyTaskSupplementAvailable"| dhub["daemonws Hub"]
  react --> sql
  react --> bus
  supp --> sql
  reconcile --> sql
```

### Daemon and agent package

```mermaid
flowchart LR
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  loop["daemon/task_supplement.go<br/>runTaskSupplementLoop, formatTaskSupplementInstruction"]:::changed
  brief["daemon/execenv/runtime_config_sections.go<br/>brief section"]:::changed
  ver["pkg/agent/version.go<br/>SupportsTaskSupplement: omp"]:::changed
  pi["pkg/agent/pi.go<br/>omp dialect: RPC, steer, session_settled"]:::changed
  loop -->|"Session.Supplement"| pi
  ver --> loop
```

### Web and desktop

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  rs["core/issues/run-steering.ts<br/>steer by default"]:::changed
  chips["views/issues/components/comment-trigger-chips.tsx"]:::changed
  marks["views/issues/components/run-input-marks.tsx<br/>👀 and ✅ with agent name"]:::new
  card["views/issues/components/comment-card.tsx"]:::changed
  qcard["views/questions/.../question-card.tsx"]:::changed
  row["views/issues/components activity row"]:::changed
  sync["core/realtime/use-realtime-sync.ts<br/>run_input:reacted"]:::changed
  rs --> chips
  marks --> card
  marks --> qcard
  marks --> row
  sync -->|"invalidate timeline, questions"| card
```

### Model

```mermaid
classDiagram
  class TaskSupplement {
    task_id UUID
    item_kind comment|answer|issue_change
    item_id UUID
    comment_id UUID nullable
    issue_id UUID
    author_id UUID
    content TEXT nullable
    status pending|delivering|delivered|failed
    delivered_at TIMESTAMPTZ
    seen_at TIMESTAMPTZ
    handled_at TIMESTAMPTZ
  }
  class Comment
  class QuestionEvent
  class ActivityLog
  class AgentTaskQueue
  TaskSupplement --> AgentTaskQueue : task_id
  TaskSupplement ..> Comment : item_id when comment
  TaskSupplement ..> QuestionEvent : item_id when answer
  TaskSupplement ..> ActivityLog : item_id when issue_change
```

### Sequence

```mermaid
sequenceDiagram
  actor B as Brian
  participant S as Go server
  participant D as Daemon
  participant O as omp (RPC)
  participant C as multica CLI in the run
  B->>S: answer Thomas's question on WORK-434
  S->>S: question_event answered, receipt (answer, item = event id, pending)
  S-->>D: daemon:task_supplement_available
  D->>S: POST /api/daemon/tasks/{id}/supplements/claim
  S-->>D: kind answer, item id, author, text
  D->>O: {"type":"steer","message":"[Multica] Brian answered ... item <id> ..."}
  D->>S: ack delivered
  O->>C: multica issue react <id> 👀
  C->>S: POST /api/tasks/{task}/inputs/{id}/reactions 👀
  S-->>B: run_input:reacted, 👀 Thomas on the answer
  O->>C: multica issue react <id> ✅
  O-->>D: session_settled, daemon closes stdin
  D->>S: complete
  S->>S: reconcile: comments without handled_at only
```

### Data flow

```mermaid
flowchart LR
  item["comment, answer event or activity row"] --> pick{"running run with<br/>task-supplement-v1 on the issue,<br/>agent is not the actor?"}
  pick -- no --> today["today's handling"]
  pick -- yes --> receipt["task_supplement row pending"] --> steer["daemon steers the run"] --> marks{"✅ when the run ends?"}
  marks -- yes --> nothing["nothing starts; answer counts as seen"]
  marks -- no --> normal["comment: follow-up run<br/>answer: stays unseen, Wake offered<br/>issue change: nothing"]
```

### Inventory

| Kind | Name | Change | Path |
|---|---|---|---|
| Migration | `task_supplement` items, content, 👀 and ✅ times | new | `server/migrations/720_task_supplement_items.up.sql`, `.down.sql` |
| Migration | Unique index on `(item_id, task_id)` | new | `server/migrations/721_task_supplement_item_task_index.up.sql`, `.down.sql` |
| Migration | Primary key on `(item_id, task_id)` | new | `server/migrations/722_task_supplement_item_task_primary_key.up.sql`, `.down.sql` |
| Migrator | Index cleanup entry | changed | `server/cmd/migrate/main.go` |
| Queries | Receipt create per kind, claim with kind and item, react, reconcile reads ✅, unseen answers leave out ✅ | changed | `server/pkg/db/queries/supplement.sql`, `comment.sql`, `question.sql` |
| Service | `RunInputService`: receipts for comments, answers and issue changes | new | `server/internal/service/run_input.go` |
| Handler | Comment steering by default for every author | changed | `server/internal/handler/task_supplement.go`, `comment.go` |
| Handler | `ReactToRunInput` | new | `server/internal/handler/run_input.go`, `server/cmd/server/router.go` |
| Service | Answers and ask-backs hand receipts to `RunInputService` | changed | `server/internal/service/question.go` |
| Listener | Issue-change rows hand receipts to `RunInputService` | changed | `server/cmd/server/activity_listeners.go` |
| Protocol | Claim response kind and item id; `run_input:reacted` event | changed | `server/pkg/protocol/messages.go`, `events.go` |
| Agent backend | omp on RPC with steer and `session_settled` | changed | `server/pkg/agent/pi.go`, `version.go` |
| Daemon | Delivered text names kind, item id and the react command | changed | `server/internal/daemon/task_supplement.go`, `client.go` |
| Brief | "While you run" section for runs with mid-run input | changed | `server/internal/daemon/execenv/runtime_config_sections.go` |
| Skill | `multica issue react` | changed | `server/internal/service/builtin_skills/multica-platform/` |
| CLI | `multica issue react` | new | `server/cmd/multica/cmd_issue.go` |
| Core | Steer by default; receipt fields; reactions in answers and activity entries; realtime event | changed | `packages/core/issues/run-steering.ts`, `packages/core/types/`, `packages/core/api/schemas.ts`, `packages/core/realtime/use-realtime-sync.ts` |
| Component | `RunInputMarks` | new | `packages/views/issues/components/run-input-marks.tsx` |
| Views | Comment card, question card, activity row show the marks | changed | `packages/views/issues/components/comment-card.tsx`, `packages/views/questions/`, `packages/views/issues/components/` |
| Docs | Comments and CLI pages | changed | `apps/docs/content/docs/comments.mdx`, `cli.mdx` |
| Living view | Server, daemon and web maps | changed | `specs/docs/system-design.md` |
| ADR | Input reaches a running run as a receipt; ✅ decides the run's end | new | `adr/0019-input-reaches-a-running-run-as-a-receipt.md` |
| ADR | Only Wake turns answers into a run; ✅ counts as seen | new | `adr/0020-only-wake-turns-answers-into-a-run-and-a-checked-answer-is-seen.md` |

## Context

Team: Backend (`uv run .../roster.py`), led by Joeri. Skuddy's hand-off on WORK-438 keeps the frontend part (marks, composer default) in this change because it shows the same behaviour; groups 7 and 8 are frontend work.

Every agent in Brian's workspace runs on the Oh-My-Pi runtime (`omp/18.10.1`). Upstream already delivers member comments into running Claude Code, Codex and Grok runs ("task supplements", MUL-7565, MUL-7631); omp is not one of them, which is the cause of what Brian sees.

What exists and is reused:

- Receipts: `task_supplement` (`server/migrations/538_task_supplement.up.sql`; primary key `(comment_id, task_id)` from `549`), capability row `task_supplement_capability` written at task start (`supplement.sql` `StartAgentTaskWithSupplement`). Claim, ack, retry, terminal settlement: `server/pkg/db/queries/supplement.sql`; handlers `server/internal/handler/task_supplement.go` (`CreateTaskSupplement`, `ClaimTaskSupplement`, `AckTaskSupplement`, `publishCommentSupplementUpdate`).
- Steering a comment: `steerCommentAgentTriggers` (`task_supplement.go:140-184`), called from `triggerTasksForComment` (`comment.go:2049`). It steers only `member` authors and only the exact turns the composer sent in `steer_task_ids` (`comment.go:1494-1498`, `1774`).
- Run end: `reconcileCommentsOnCompletion` (`handler/daemon.go:4510-4694`) replays planned comments through `comment.sql:375-400`, whose `NOT EXISTS (... task_supplement ...)` drops every steered comment, delivered or not.
- Daemon: `runTaskSupplementLoop` (`daemon/task_supplement.go:132-182`) claims one receipt at a time, calls `Session.Supplement` once `SupplementReady`, acks. `formatTaskSupplementInstruction` (`:90`) builds the text. Negotiation: `agent.SupportsTaskSupplement` (`pkg/agent/version.go:114-134`) at `StartTask` (`daemon.go:8050-8065`).
- omp backend: `piBackend.Execute` (`pkg/agent/pi.go:352-755`) runs `omp -p --mode json --session <path>` and writes the prompt then EOF (`:406-428`, `buildPiArgs` `:995-1030`); no `Supplement`. omp's RPC mode (`omp --mode rpc --no-ui`) takes `prompt`, `steer` and reports `prompt_result` and `session_settled`; closing stdin shuts it down (omp docs `rpc.md` "Yield vs settled", "Transport and Framing").
- Questions: `QuestionService.Answer` (`service/question.go:597-634`) writes `question_answer` and a `question_event` (`answered`, `answer_edited`, `ask_back`) and starts no run (ADR-0003). Unseen answers: `CountUnseenQuestionAnswers` (`question.sql:200-223`); woken claim input: `loadClaimQuestionAnswers` (`daemon.go:3727-3799`).
- Issue changes: `activity_listeners.go:63-280` writes one `activity_log` row per changed field of `issue:updated` (`status_changed`, `priority_changed`, `assignee_changed`, `start_date_changed`, `due_date_changed`, `title_changed`, `description_updated`).
- Reactions: `comment_reaction` and `issue_reaction` (migrations 026, 027) are people-and-agent emoji reactions with no notion of a run; they stay as they are.
- CLI: `issue comment` commands at `cmd_issue.go:321-376`; agent auth through `MULTICA_TOKEN` and `MULTICA_TASK_ID` (`daemon.go:178-198`). Brief: `execenv.InjectRuntimeConfig` (`runtime_config.go:140-147`, `runtime_config_sections.go`); built-in skill `server/internal/service/builtin_skills/multica-platform/`.
- Frontend: `packages/core/issues/run-steering.ts` (`resolveRecipientAction`, `steerByDefault`), `comment-trigger-chips.tsx:340`, receipts on `comment-card.tsx:680`, `1063`; question cards `question-card.tsx:134-149`; realtime `use-realtime-sync.ts:1088-1345`.

## Goals / Non-Goals

**Goals:**

- Always: an item reaches only runs in `running` with `task-supplement-v1`; never the actor's own agent's run; a queued or starting run gets comments at claim as today.
- Always: one receipt row per item and run, the only record of what a run received and of its 👀 and ✅.
- Always: run-end handling reads only ✅ (`handled_at`).
- Never: an answer starts a run (ADR-0003, restated in ADR-0020).
- Never: a person's reaction counts as an agent's ✅.

**Non-Goals:**

- Mid-run input for pi and the other runtimes without it today.
- Marks on a run's own starting input.
- Attachments, labels, sub-issues and reactions as delivered issue changes.

## Decisions

1. **Receipts for every kind live in `task_supplement`.** The table gains `item_kind` (`comment`, `answer`, `issue_change`), `item_id` (the comment id, the `question_event` id or the `activity_log` id), `content` (the text for answers and changes, rendered at creation), `seen_at` and `handled_at`; `comment_id` becomes nullable and stays set for comments, so upstream's comment queries keep working. The primary key moves to `(item_id, task_id)`, built `CONCURRENTLY` and swapped as `549` did. Alternative: a fork-owned `run_input` table beside `task_supplement`. Rejected: two receipt stores, two claim paths in the daemon and two places to read ✅. Upstream merges touching `task_supplement` may conflict; the columns are additive to keep them small.
2. **One service creates receipts.** `RunInputService.Deliver(ctx, issueID, actor, item, agentIDs)` finds the issue's tasks in `running` with `task-supplement-v1` whose agent is in `agentIDs` and is not the actor, inserts a `pending` receipt per task in the task-row lock order `CreateTaskSupplement` uses, and calls `NotifyTaskSupplementAvailable`. Who an item is for: a comment, the agents `computeCommentAgentTriggers` returns for it (mention, assignee, thread), called through `steerCommentAgentTriggers`; an answer, edited answer or ask-back, the question's asking agent, called by `QuestionService` after the event commits; an issue change, every agent with a running task on the issue, called by the activity listener after each issue-change row. A comment with attachments gets no receipt (text only, as today).
3. **Steer is the server's default.** A comment request without `steer_task_ids` (the CLI, agents, older clients) steers every running, supplement-capable recipient it triggers, for any author type. A request with `steer_task_ids`, also an empty list, is the composer's explicit choice and is followed as today. The composer always sends the list, with every running recipient at **Add to current run** by default (`steerByDefault: true`).
4. **omp runs on its RPC dialect.** For the `omp` identity only, `piBackend` starts `omp --mode rpc --no-ui --session <path>`, writes one `prompt` frame, ignores `response` and `prompt_result` frames in the event reader, closes stdin on `session_settled`, and sets `Session.Supplement` to write a `steer` frame and `SupplementReady` after `agent_start`. `SupportsTaskSupplement("omp", v)` returns true from `18.10.1`, the version on Brian's Mac. pi keeps print mode.
5. **Delivered text names the item.** The claim response adds `item_kind` and `item_id`; `formatTaskSupplementInstruction` writes, for example: `[Multica] Brian commented on WORK-434 (item 01a1…): <text>. Mark it: multica issue react 01a1… 👀 when seen, ✅ once handled.`
6. **React is a run-scoped endpoint.** `POST /api/tasks/{taskId}/inputs/{itemId}/reactions` with `{"emoji":"👀"|"✅"}` under agent auth; the task token's task must be `taskId` and a receipt `(itemId, taskId)` must exist with status `delivered`. It sets `seen_at` or `handled_at` once (`COALESCE`), then publishes `run_input:reacted` with the issue id, item kind and item id. `multica issue react <item> <emoji>` reads `MULTICA_TASK_ID` and fails outside a run with "react works only inside an agent's run".
7. **Run end reads ✅.** `comment.sql`'s steered-comment exclusion keeps only receipts with `handled_at IS NOT NULL`, so a delivered comment without ✅ replays like any comment posted during the run. `FailTask` runs the same reconciliation after its transition, so a failed run's comments without ✅ replay too; an auto-retry it queued absorbs them through the existing merge into a queued task. Answers without ✅ stay unseen (decision 8, ADR-0020); issue changes start nothing.
8. **Unseen answers leave out ✅.** `CountUnseenQuestionAnswers` and `loadClaimQuestionAnswers` skip an answer whose latest `answered` or `answer_edited` event has a receipt with `handled_at` for a task of the same agent. Wake and the woken input stay one query each (ADR-0003, ADR-0020).
9. **Marks render from receipts.** Comment receipts gain `seen_at` and `handled_at`; question answers and activity entries gain `run_marks: [{agent_id, seen_at, handled_at}]`. One `RunInputMarks` component shows 👀 and ✅ with the agent's name. `run_input:reacted` invalidates the issue's timeline and question queries.
10. **The brief tells it only where it applies.** `runtime_config_sections.go` adds a "While you run" section when the task starts with `task-supplement-v1`; the `multica-platform` skill documents `multica issue react`.

## Risks / Trade-offs

- [omp RPC pins neutral defaults (async jobs, bash auto-background, advisor) that print mode does not] -> the runtime's explicit `config.yml` stays authoritative per omp's RPC docs; the backend test compares the frames of a print-mode and an RPC-mode run of the same prompt.
- [A run that never yields keeps omp alive] -> the existing idle watchdog and cancel path in `executeAndDrain` still kill the process; RPC adds no new wait.
- [Agents that ignore the brief never ✅] -> every delivered comment then replays as today: no input is lost.
- [A busy issue floods a run with changes] -> receipts are claimed one at a time and steer folds into omp's steering queue; the brief tells the agent a change can be 👀 and ✅ at once.
- [Upstream merges on `task_supplement`] -> additive columns; the primary-key swap follows upstream's own `549` pattern.

## Migration Plan

Migrations 720 to 722 run with `make migrate`; existing rows get `item_kind = 'comment'` and `item_id = comment_id`. Down migrations restore the `(comment_id, task_id)` key after deleting non-comment rows. A daemon without the new claim fields still delivers comments; answers and changes need the daemon of this change. Release: `make local-release` after merge.

## Open Questions

The plan follows the first option of each entry; another answer changes the part named in it.

1. **Who hears a comment or an answer.** Thomas runs on WORK-434, whose assignee is Skuddy. Brian replies in Skuddy's thread there, or answers Skuddy's question, and does not mention Thomas.
   - (a) Only the agent it is for (mentioned, assignee, thread starter, asker) hears it; Thomas does not. Keeps runs on their own work and follows who a comment wakes today.
   - (b) Every agent running on the issue hears it, Thomas too. Matches "any running agent on the issue" literally; changes decision 2 and live-run-input.
2. **Edited comments.** Thomas's run already received Brian's comment; Brian then edits it.
   - (a) The edit does not reach the run. Most edits fix typos, and the run already has the meaning.
   - (b) The edited text reaches the run as a new item to mark 👀 and ✅. Matches "any change to a thread"; adds an item kind and a spec requirement.
3. **Which issue changes reach a run.** Brian changes WORK-434 while Thomas runs on it.
   - (a) Status, priority, assignee, title, description, start date and due date. These are the changes the activity log records today.
   - (b) Also labels, attachments and sub-issues. Matches "any change to a task"; adds three new change sources.
