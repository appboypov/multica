# ADR Review Manifest

- Status: completed
- Review date: 2026-10-15

## Review Summary

ADR review completed for this change. No in-force ADR conflicts with it.

## In-Force ADRs Reviewed

- `adr/0017-an-agents-sessions-are-its-followed-sessions-and-its-running-tasks.md` (from `agent-session-model`): this change builds what it decides: the daemon registers and follows sessions started by hand, runs come from the queue.
- `adr/0001-runtime-machine-files-are-read-by-the-daemon.md`: kept; the daemon reads each session's process, folder and branch.
- `adr/0007-worktrees-are-made-by-the-daemon-and-recorded-by-the-server.md`, `adr/0014-a-worklanes-issues-work-in-its-worktree-on-their-own-branch.md`: kept; a run's repository and branch come from the recorded issue worktree.
- The other in-force ADRs concern questions, projects, worklanes, releases and issue templates: unrelated.

## New Durable ADRs Created

- None - `adr/0017` was recorded with `agent-session-model`.
