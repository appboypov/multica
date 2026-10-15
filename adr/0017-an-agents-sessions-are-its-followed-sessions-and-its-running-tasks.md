# ADR-0017: An agent's sessions are its followed sessions and its running tasks

- Status: accepted
- Date: 2026-10-15
- Supersedes: none

## Context

Brian wants each agent's active sessions in Multica, wherever they run: sessions he starts by hand with a persona command, in Herdr or in a plain terminal, and Multica runs (WORK-1057). The server alone cannot see a terminal close or a pane change; the queue already knows every run from claim to its end.

## Decision

- A session started by hand is a row the daemon of its machine registers and follows. The daemon ends it on Herdr's pane events and on a 30-second sweep of its process and branch, and drops what ended while it was down when it starts.
- A run is never stored as a session. The list reads the agent's `dispatched` and `running` tasks at read time, with the work folder on the task. For an issue in a worklane, including `main`, repository comes from the recorded lane and branch is the lowercase issue key (ADR-0014), not the lane's current branch. Both lane and repository must be in the agent's workspace; an unresolved reference leaves both fields null. For an issue without a lane, repository and branch come from its active issue worktree at the task's folder, else its only active worktree; several active worktrees elsewhere leave both empty (ADR-0007).
- Sessions of a daemon that is not online are not listed.
- The daemon acts only on Herdr panes of sessions it registered, so its own pane never stands for a run.

## Consequences

- One source per kind of session: a run's entry cannot outlive its task, and no sync between the queue and a session table exists.
- A session started by hand needs a running daemon on its machine to be listed.
- A new way to start an agent by hand registers through `multica agent register-session`; a new runtime needs nothing, since its runs come from the queue.
