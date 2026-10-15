# ADR-0020: Only Wake turns answers into a run, and an answer its run marked ✅ is seen

- Status: accepted, supersedes ADR-0003
- Date: 2026-10-14
- Supersedes: ADR-0003

## Context

ADR-0003 decided that questions never start agents and that "unseen" has one definition: the answers to an agent's questions updated after the claim of that agent's last run on the issue. Answers given while the run runs stay unseen until the run ends, and Wake then offers them.

With ADR-0019 an answer reaches the asking agent's running run at its next step, and the agent marks it 👀 and ✅. An answer the run marked ✅ was dealt with in that run, and offering Wake for it would start a run that has nothing to do. The rest of ADR-0003 still holds.

## Decision

- The only path from questions to a run stays an explicit Wake by a member, as ADR-0003 decided; a delivered answer, ✅ or not, never queues a run.
- "Unseen" keeps one definition in one query: the answers to that agent's questions on the issue updated after the claim of that agent's last run on the issue, as ADR-0003 counts them, minus each answer whose latest `answered` or `answer_edited` event has a receipt with `handled_at` for a run of that agent.
- Wake's count and the woken run's input use that same definition.

## Consequences

- An answer the run received and did not mark ✅ is offered by Wake once the run ends, as before.
- An answer edited after its ✅ is unseen again: the new event has no receipt with `handled_at`.
- Wake is still not offered while the agent has a queued, dispatched, running or waiting-for-local-directory task on the issue.
- Archived agents and agents without a runtime cannot be woken; an unavailable asker's card still counts its unseen answers.
