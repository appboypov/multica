## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0006: A repository belongs to a project and holds its worklanes (supersedes ADR-0004) | new | `adr/0006-repositories-hold-worklanes.md` |
| ADR | ADR-0007: Worktrees are made by the daemon and recorded by the server | new | `adr/0007-worktrees-are-made-by-the-daemon-and-recorded-by-the-server.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. Both new ADRs bind more than this change:

- ADR-0006 fixes the repository and worklane levels that UJ-3 to UJ-8 extend, and the worktree path for projects with and without an organisation. It supersedes ADR-0004, whose Decision made the organisation's key the first folder of every worktree path; Brian's P4 answers for UJ-2 give a project without an organisation worktrees at `~/Worktrees/{repo}/{folder}`.
- ADR-0007 fixes that only the daemon touches a runtime's disk for worktrees, that the server records each worktree, and that a worktree goes only when nothing would be lost. UJ-3 makes worklane worktrees and UJ-6 switches them after a merge under the same rules.

Numbering continues after ADR-0005, which lives on the UJ-1 branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: binding. The checkout check, the clone and issue worktrees run on the daemon through a heartbeat-carried request, the server stores only the absolute paths the daemon resolved, and only the runtime's owner may ask for a check.
- ADR-0002 Questions are their own aggregate with typed answers and links: this change adds no question link kind.
- ADR-0003 Only Wake turns answers into a run: not touched.
- ADR-0004 Organisations hold projects: superseded by ADR-0006, which keeps its organisation model.
- ADR-0005 Project status and dates stay stored, and the web and desktop apps no longer use them: binding. The project page and project rows this change touches show no status or dates.

## New Durable ADRs Created

- `adr/0006-repositories-hold-worklanes.md`
- `adr/0007-worktrees-are-made-by-the-daemon-and-recorded-by-the-server.md`
