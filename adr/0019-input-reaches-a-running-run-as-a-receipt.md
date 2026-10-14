# ADR-0019: Input reaches a running run as a receipt, and ✅ decides what the run's end hands on

- Status: accepted
- Date: 2026-10-14
- Supersedes: none

## Context

Brian answers questions, comments and changes issues while an agent runs on the issue, and expects the run to hear it at once and to show that it saw (👀) and handled (✅) each item. Upstream steers only comments that a member's composer explicitly sends into a running Claude Code, Codex or Grok turn, through `task_supplement` receipts (MUL-7565, MUL-7631). Every agent in Brian's workspace runs on omp, which upstream gives no input after the start. Answers, ask-backs and issue changes have no path into a run at all, and a run's end drops every steered comment from its follow-up handling, whether the agent dealt with it or not.

## Decision

- Every item that reaches a running run is a row of `task_supplement`, one per item and run, keyed `(item_id, task_id)`, with `item_kind` `comment`, `answer` or `issue_change`. There is no second receipt store.
- One server service creates receipts for all kinds. An item reaches only tasks in `running` that negotiated `task-supplement-v1`, and never the actor's own agent. A queued or starting run gets its input at claim, as before.
- A comment steers its running recipients by default, for every author type; the composer can still choose another action per recipient.
- The run's agent marks a received item through the run-scoped endpoint behind `multica issue react`, which sets the receipt's `seen_at` (👀) or `handled_at` (✅) once. Only the run that holds the receipt can mark it.
- When a run ends, completed or failed, only `handled_at` decides: a received item with ✅ starts nothing; one without gets the handling it gets when no run received it.
- omp takes mid-run input through its RPC mode's `steer`; a runtime without mid-run input keeps the item for the run's end.

## Consequences

- A new kind of input that should reach running runs adds an `item_kind` and a call to the service; it adds no table, endpoint or daemon path.
- People's emoji reactions (`comment_reaction`, `issue_reaction`) stay separate and never count as ✅.
- An agent that never marks ✅ gets the same follow-up runs it got before: no input is lost, at the cost of a run that redoes handled work.
- Upstream changes to `task_supplement` merge against additive columns and the moved primary key; a sync that touches the table checks both.
- omp runs use RPC mode, which pins some neutral defaults; the runtime's explicit config stays authoritative.
