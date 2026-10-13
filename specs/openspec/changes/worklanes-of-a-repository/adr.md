## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0008: Worklane activity is kept in the activity log | new | `adr/0008-worklane-activity-is-kept-in-the-activity-log.md` |
| ADR | ADR-0009: The server keeps worklane and issue worktree folders apart | new | `adr/0009-the-server-keeps-worklane-and-issue-worktree-folders-apart.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. Both new ADRs bind more than this change:

- ADR-0008 fixes where a worklane's activity lives and how repository-wide entries reach every worklane. UJ-4, UJ-6 and UJ-8 add their entries to it.
- ADR-0009 fixes how worklane folders and issue worktree folders, which share `~/Worktrees/{organisation key}/{repo}/`, never point at the same worktree. It binds UJ-2's issue worktrees and UJ-6's worklane switch.

Numbering continues after ADR-0007, which lives on the UJ-2 branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: binding. Fetch, worktree add, install and remove run on the daemon through a heartbeat-carried request; the server stores only the absolute path the daemon resolved.
- ADR-0002 Questions are their own aggregate with typed answers and links: not touched.
- ADR-0003 Only Wake turns answers into a run: not touched.
- ADR-0005 Project status and dates stay stored, and the web and desktop apps no longer use them: binding. The worklane page and the rows this change touches show no project status or dates.
- ADR-0006 A repository belongs to a project and holds its worklanes: binding. Worklanes, `main`, the worktree path with and without an organisation, and the path stored when the worktree is made.
- ADR-0007 Worktrees are made by the daemon and recorded by the server: binding. Create and Remove are requests the daemon answers; Remove goes only when nothing would be lost; the worktree path is locked while install runs.

## New Durable ADRs Created

- `adr/0008-worklane-activity-is-kept-in-the-activity-log.md`
- `adr/0009-the-server-keeps-worklane-and-issue-worktree-folders-apart.md`
