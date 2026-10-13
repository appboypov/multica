# UJ-1 data-flow prototype

## At a glance

Data flow of journey UJ-1 "Answer an agent's question and wake it", per step: its input, the agent's action, the stores it writes or reads, and its output, as the prototype shows them.

```mermaid
flowchart TB
  subgraph S1["1 · Pieter asks during run #3"]
    direction LR
    I1["Input<br/>multica question ask --type single_choice<br/>POST /api/questions, task token of run #3"] --> A1["Agent action<br/>Pieter asks, prints the id,<br/>ends run #3"] --> D1[("Store · write<br/>question open, question_link issue + task,<br/>question_answerer Brian, question_event asked")] --> O1["Output<br/>201 Question, wake_available false<br/>bus question:created"]
  end
  subgraph S2["2 · Brian is told"]
    direction LR
    I2["Input<br/>bus question:created"] --> A2["Agent action<br/>none: no active task"] --> D2[("Store · write<br/>inbox_item question_asked for Brian<br/>agent_task_queue: no new row")] --> O2["Output<br/>WS inbox:new, question:created"]
  end
  subgraph S3["3 · Brian opens WORK-412 from the inbox"]
    direction LR
    I3["Input<br/>GET /api/issues/{id}/questions<br/>highlightQuestionId"] --> A3["Agent action<br/>none"] --> D3[("Store · read<br/>question tables, unseen query 0,<br/>HasActiveTaskForIssueAndAgent false")] --> O3["Output<br/>card open, unseen 0, wake_available false<br/>ended run #3 with a question"]
  end
  subgraph S4["4 · Brian accepts the suggestion"]
    direction LR
    I4["Input<br/>POST /api/questions/{id}/answer<br/>value option Per line, ValidateAnswer"] --> A4["Agent action<br/>none: answering starts no run"] --> D4[("Store · write<br/>question_answer, status answered,<br/>question_event answered<br/>agent_task_queue: no new row")] --> O4["Output<br/>200 Question, unseen 1, wake_available true<br/>bus question:updated answered"]
  end
  subgraph S5["5 · Brian wakes Pieter"]
    direction LR
    I5["Input<br/>POST /api/questions/{id}/wake"] --> A5["Agent action<br/>server queues run #4<br/>EnqueueTaskForQuestionWake"] --> D5[("Store · write<br/>agent_task_queue evidence question_wake,<br/>activity_log question_wake,<br/>question_event woken")] --> O5["Output<br/>202 task_id, answer_count 1<br/>bus question:updated woken"]
  end
  subgraph S6["6 · Pieter's new run reads the answer"]
    direction LR
    I6["Input<br/>daemon claims run #4"] --> A6["Agent action<br/>Pieter reads ANSWERS,<br/>goes on with Per line"] --> D6[("Store · read + write<br/>unseen answers since run #3,<br/>run #4 running: new anchor")] --> O6["Output<br/>claim with question_answers,<br/>prompt with ANSWERS block"]
  end
  S1 --> S2 --> S3 --> S4 --> S5 --> S6
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Prototype | UJ-1 data-flow prototype (MagicPath source) | new | `prototypes/web/issue-questions/data-flow/src/` |
| Export | One PNG per journey step | new | `prototypes/web/issue-questions/data-flow/*.png` |
| Doc | This README | new | `prototypes/web/issue-questions/data-flow/README.md` |
| Plan | Task 4.1 ticked | changed | `specs/openspec/changes/answer-an-agents-question-and-wake-it/tasks.md` |

## The prototype

MagicPath: https://designs.magicpath.ai/v1/keen-stone-4126

An interactive view without product UI. The rail on the left lists the six steps; Back, Next and the arrow keys move between them. Each step shows what happens, its input, the agent's action, the stores it writes or reads with their rows, its output, and what to click next. The Live state panel on the right follows the question through the journey and marks what the step changed. Its last row, "Runs started since the ask", is the journey's acceptance check: it stays 0 until Brian presses Wake.

`src/` is the submitted MagicPath component source; the PNGs are screenshots of the hosted build at 1440 wide, one per step: `1-ask.png`, `2-told.png`, `3-open-issue.png`, `4-accept-suggestion.png`, `5-wake.png`, `6-woken-run.png`.

The names and values come from the change's `design.md` ("Journey sequence and data flow", decisions 1 to 5 and 7) and the J2 data models merged in appboypov/multica#12: `packages/core/types/question.ts`, `packages/core/types/inbox.ts`, `server/pkg/protocol/events.go`, `server/internal/service/question_actions.go` and `server/migrations/566_question.up.sql`. The ids, issue title and times are example data.
