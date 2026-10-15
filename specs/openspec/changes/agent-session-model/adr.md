# ADR Review Manifest

- Status: completed
- Review date: 2026-10-15

## Review Summary

ADR review completed for this change. No in-force ADR conflicts with it.

## In-Force ADRs Reviewed

- `adr/0001-runtime-machine-files-are-read-by-the-daemon.md`: kept; the daemon reads each session's process, folder and branch.
- `adr/0007-worktrees-are-made-by-the-daemon-and-recorded-by-the-server.md`: kept; a run's repository and branch come from the recorded issue worktree.
- `adr/0009-the-server-keeps-worklane-and-issue-worktree-folders-apart.md`, `adr/0014-a-worklanes-issues-work-in-its-worktree-on-their-own-branch.md`: kept; the list reads the issue worktree rows as they are.
- `adr/0005-whether-a-run-may-ask-is-decided-at-claim.md`: unrelated; a run's session adds nothing to its claim.
- `adr/0002`, `0003`, `0004-question-sets-group-questions-open-is-per-question`, `0005-project-status-and-dates-stay-stored-but-unseen`, `0006`, `0007-a-question-reaches-its-parents-at-read-time`, `0008`, `0010` to `0013`, `0015`, `0016`: unrelated to agents, runtimes or sessions.

## New Durable ADRs Created

- `adr/0017-an-agents-sessions-are-its-followed-sessions-and-its-running-tasks.md`
