# ADR-0018: A wake continues where the question was asked

- Status: accepted, supersedes ADR-0003
- Date: 2026-10-15
- Supersedes: ADR-0003 (`0003-only-wake-turns-answers-into-a-run.md`)

## Context

ADR-0003 makes an explicit Wake by a member the only path from questions to a run, always as a run on the link's issue, with "unseen" defined per agent and issue. Since then Brian decided that sending the last open question of an ask with **Wake {agent}** checked wakes the asking agent and that an ask back wakes it (WORK-429), and on WORK-447 (2026-10-15) that agents ask in chats, that every wake of a chat question's asking agent continues that chat, and that a member can cancel a question, which reaches the woken run like an answer. A chat turn has no issue, so neither the issue run nor the issue-anchored "unseen" fits a chat question.

## Decision

- The paths from questions to a run are a member's **Wake**, an answer or cancel sent with **Wake {agent}** checked, and an ask back. Each calls `QuestionService.Wake`; there is no other enqueue path. Question events are never comment triggers and never part of the issue-wakeup event vocabulary.
- A wake has a scope: the chat a question's asking agent asked it in, when the asking run (`asked_in_task_id`) is a chat turn, else the link's issue. For a chat scope `Wake` enqueues the agent's next turn in that chat, a chat task without a user message and with trigger evidence `question_wake`; for an issue scope a run on the issue, as before.
- "Unseen" has one definition, in one set of queries parameterised by scope: the answers, ask backs and cancels on that agent's questions in the scope, updated after the claim (`dispatched_at`) of its latest run that started in that scope, outside the claiming run's `retry_of_task_id` chain. The same queries decide whether Wake is offered and fill the woken run's `[ANSWERS]` block at claim.
- A chat turn always receives the unseen answers, ask backs and cancels of its chat at claim, whatever started it, because every started chat turn moves the chat's cutoff.
- Answers, ask backs and cancels appear in a per-turn prompt block, never in the cached brief; an empty payload adds no block.

## Consequences

- A new place that offers Wake calls the same endpoint and reads the same unseen count; it adds no enqueue path.
- Agent assignees of links and parents are woken on their issue through the same call with their own agent id.
- Wake is not offered while the agent has a queued, dispatched or running task in the scope; answers given meanwhile stay unseen and ride along in the next turn in that scope.
- A test that sends, edits, cancels and asks back without Wake and finds no queued task guards the rule.
