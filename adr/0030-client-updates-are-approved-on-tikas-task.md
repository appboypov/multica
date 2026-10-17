# ADR-0030: Client updates are approved on Tika's task

- Status: accepted, supersedes ADR-0028
- Supersedes: ADR-0028
- Date: 2026-10-15

## Context

ADR-0028 has Hermila ask Brian to approve each draft with an approval question on the work's issue, and make Tika's task only after his Approve. An approval question shows plain text only, while a Multica task renders Markdown with images. Brian passed the Approach "Hermila makes Tika's task and asks Brian to approve it": an update can carry one visual of the work, and Brian approves it where it renders, on the task Tika posts from.

## Decision

- Multica holds MadSpec's planning and Tasks, as `multica-platform` `references/madspec-planning.md` says.
- `multica_sync.py` and `multica_projects.py` stay in Brian's Multica fork `~/Repos/Forks/multica`, in `madspec-sync/`, and run only by hand.
- One Multica agent, Hermila (Client Updates), writes every client update. An agent that reaches an update moment mentions her on the work's issue; her whole job is the skill `madspec-client-updates`.
- Each draft is Tika's task in the client's project, holding the update exactly as the client gets it. It has no assignee while it waits, and is never assigned to Brian.
- Hermila asks Brian to approve the draft with a Multica approval question on that task, linked to the work's issue. His answer is his word for that post and wakes her on the task. On Approve she assigns that same task to Tika.
- Tika, who owns every contact chat (agents ADR-0002 "Tika owns every contact chat"), is a Multica member, never an agent, and posts approved updates from the Multica tasks assigned to her.
- A session's Multica login comes from its folder: a `.multica-profile` file names the CLI profile, Skuddy's without one.

## Consequences

- MadSpec skills name Multica where they plan, hand off and post updates.
- What Brian approves and what Tika posts are one Multica task: no copy between them.
- Tika's session hears her own Multica inbox, and never Brian's; a task reaches it only once Brian approved it.
- Multica starts no Tika run: assigning her a task only reaches her session.
