# ADR-0028: Client updates go through Hermila and Tika in Multica

- Status: superseded by ADR-0030
- Supersedes: madspec ADR-0005
- Date: 2026-10-15

## Context

madspec ADR-0005 kept MadSpec out of Multica: no skill named it, and the tools of `omp-multica-listen` ran only when Brian asked for Multica. On 15 October 2026 Brian put MadSpec's planning back in Multica: lanes, releases, issues, steps and client waits, with the Multica issue as the Task of every hand-off (`multica-platform` `references/madspec-planning.md`). Brian then passed the Approach "Approved client updates through the Updates agent and Tika", which adds a Multica agent and a member.

## Decision

- Multica holds MadSpec's planning and Tasks, as `multica-platform` `references/madspec-planning.md` says.
- `multica_sync.py` and `multica_projects.py` stay in Brian's Multica fork `~/Repos/Forks/multica`, in `madspec-sync/`, and run only by hand.
- One Multica agent, Hermila (Client Updates), writes every client update. An agent that reaches an update moment mentions her on the work's issue; her whole job is the skill `madspec-client-updates`.
- Hermila asks Brian to approve each draft with a Multica approval question on the work's issue. His answer is his word for that post and wakes her, as it wakes every agent.
- Tika, who owns every contact chat (agents ADR-0002 "Tika owns every contact chat"), is a Multica member, never an agent, and posts approved updates from the Multica tasks assigned to her.
- A session's Multica login comes from its folder: a `.multica-profile` file names the CLI profile, Skuddy's without one.

## Consequences

- MadSpec skills name Multica where they plan, hand off and post updates.
- Tika's session hears her own Multica inbox, and never Brian's.
- Multica starts no Tika run: assigning her a task only reaches her session.
