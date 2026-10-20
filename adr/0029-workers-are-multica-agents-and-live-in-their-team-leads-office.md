# ADR-0029: Workers are Multica agents and live in their team lead's office

- Status: accepted; supersedes agents ADR-0001 in part and madspec ADR-0005 in part; superseded in part by WORK-1957: agents get their prompts only from Multica, run on the generic Oh-My-Pi runtime, and no sync writes them
- Date: 2026-10-17
- Supersedes: agents ADR-0001 for worker skill ownership; madspec ADR-0005 for builders and reviewers running as omp subagents

## Context

[Agents ADR-0001](../../../Agents/adr/0001-persona-skills-live-in-their-agents-office.md) puts persona skills in their offices and keeps shared skills in MadSpec. [madspec ADR-0005](../../../Ventures/madspec/adr/0005-madspec-runs-on-its-own.md) makes builders and reviewers omp subagents of the persona doing the step. Teams need workers with their own tasks, models and instructions. Keeping workers inside a lead's session would tie their work to that session. Keeping their instructions in MadSpec would split each team's tools and working rules across repositories.

## Decision

- MadSpec holds the what: the roster, the flow and the specs. Each agent's office holds its how: its instructions and tooling.
- Worker skills live under `skills/` in MadSpec, beside the other shared skills.
- Worker agent files live under `agents/` in the team lead's office. Their frontmatter holds each worker's name, models and skills; their body holds its instructions. The roster finds those files.
- Each worker is a Multica agent on the omp runtime, with its own model. `madspec-sync/` in Brian's Multica fork, `~/Repos/Forks/multica`, syncs workers by hand. No hook or schedule runs that sync.
- A lead gives a worker a Multica sub-issue, not an omp subagent task. Levels, models, team ownership and the hand-off steps live in [Workers](../../../Ventures/madspec/skills/madspec/help/workers.md). That page also names the helpers that stay in the calling session.

## Consequences

- A worker's instructions are edited and committed in its lead's office. Its skills, the shared workflow and the specs are edited and committed in MadSpec.
- A worker's task and result remain on its Multica issue instead of inside its lead's session.
- MadSpec's office links need the offices at their absolute paths. A checkout without those offices cannot resolve the links.
- A change to a worker's source files reaches its Multica agent when the manual sync runs.
