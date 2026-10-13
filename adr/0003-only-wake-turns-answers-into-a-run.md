# ADR-0003: Only Wake turns answers into a run

- Status: accepted
- Date: 2026-10-12
- Supersedes: none

## Context

Brian decided that questions never start agents: asking, answering, partly answering and editing an answer start no run, and a person wakes an agent once it has answers it has not seen. Multica starts runs from several paths (assignment, comment triggers, mentions, issue wakeups, autopilot, retries), and a new event type could easily be wired into one of them. Every questions journey (UJ-1 to UJ-4) offers Wake in another place: the card, a question set's footer, focus mode, the end of a link's cards.

## Decision

The only path from questions to a run is an explicit Wake by a member, which calls `TaskService` to enqueue the chosen agent on the link's issue with trigger evidence `question_wake`. Question events are never comment triggers and never part of the issue-wakeup event vocabulary. "Unseen" has one definition, in one query: the answers to that agent's questions on the issue updated after the start of its latest started run there. The same query decides whether Wake is offered and fills the woken run's input at claim, as a per-turn prompt block, never in the cached brief.

## Consequences

- A new place that offers Wake calls the same endpoint and reads the same unseen count; it adds no enqueue path.
- UJ-4's agent assignees of links and parents are woken through the same call with their own agent id.
- Wake is not offered while the agent has a queued or running task on the issue; answers given during a run stay unseen and Wake returns after it.
- A test that sends, edits and asks back on a question and finds no queued task guards the rule.
