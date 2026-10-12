# ADR-0005: Whether a run may ask is decided once, at claim

- Status: accepted
- Date: 2026-10-12
- Supersedes: none

## Context

Brian decided that whether agents may ask questions is a workspace default each issue can change at any time, that a change applies from the next run, that a single run has no setting of its own, and that an agent that may not ask is told nothing about questions. Several parts read the decision: the brief's Questions section, `multica --help`, the built-in skills, the `[ANSWERS]` tail line and `POST /api/questions`, which every ask of UJ-1 and UJ-2 goes through. Each reading the live issue would let a run lose its tool mid-run, and would let two parts of one run disagree.

## Decision

The decision is made once per run, when its task is claimed: `service.ResolveAgentQuestions(issue.agent_questions, workspace.settings)`, the issue's value when set, else the workspace default, absent meaning On; a task without an issue is off. The claim stores it on `agent_task_queue.questions_enabled` under the claim CAS and sends it to the daemon as `questions_enabled`. The brief, the daemon's environment, the CLI and the skill variant follow the claimed value; `POST /api/questions` refuses a task whose stored value is false. Product-owned text an off run must not see sits between `<!-- questions -->` markers in built-in skills, and an off run gets the skill variant without it, under its own skill id.

## Consequences

- A change of the setting reaches the next claimed task, never a running one.
- A later journey that adds text about questions to the brief, the CLI or a built-in skill puts it behind the claimed value or the markers; one that adds another way to ask checks the stored value.
- Tasks claimed before this change have `questions_enabled` false: they cannot ask, which fails closed.
- Built-in skills with a questions block have two ids; the daemon caches both by hash under one directory name.
