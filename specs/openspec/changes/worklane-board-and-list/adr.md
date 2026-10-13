## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0014: A worklane's issues work in its worktree on their own branch | new | `adr/0014-a-worklanes-issues-work-in-its-worktree-on-their-own-branch.md` |
| ADR | ADR-0015: The database keeps an issue's done date | new | `adr/0015-the-database-keeps-an-issues-done-date.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. Both new ADRs bind more than this change:

- ADR-0014 fixes where an agent works a worklane's issue, that a worklane runs one task at a time and only on its own machine, that its worktree changes branch only when nothing would be lost, and that Busy or Free and the next issue are computed from the branch the daemon reports. UJ-7 shows now or next on worklane rows from it.
- ADR-0015 fixes that an issue's done date is kept by a trigger on every status write. UJ-8 derives a release's ship date from it.

Numbering continues after ADR-0013, which lives on the UJ-5 branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: binding. Only the daemon reads and switches the worktree's branch, and the server learns the branch from its reports.
- ADR-0002 Questions are their own aggregate with typed answers and links: not touched.
- ADR-0003 Only Wake turns answers into a run: not touched.
- ADR-0004 Organisations hold projects, and an organisation's key names its worktree folder: not touched. A worklane's path is the one UJ-3 recorded.
- ADR-0005 Project status and dates stay stored, and the web and desktop apps no longer use them: not touched. The done date is an issue date.
- ADR-0006 A repository belongs to a project and holds its worklanes: binding. An issue of `main` works in the repository's checkout, which is `main`'s worktree.
- ADR-0007 Worktrees are made by the daemon and recorded by the server: binding. The daemon switches the worklane's branch and the server records it in `worklane.branch`. No `issue_worktree` row is made for an issue in a worklane.
- ADR-0008 Worklane activity is kept in the activity log: binding. `worklane_switched` and `worklane_kept` are worklane rows.
- ADR-0009 The server keeps worklane and issue worktree folders apart: kept. It still holds for issues without a worklane, the only issues that get an issue worktree.
- ADR-0010 The server keeps each repository's release line: binding. The next issue follows the line's order by `release.position`.
- ADR-0011 A worklane shows repository activity from its creation on: not touched.
- ADR-0012 The server holds the issue templates, and a type is set once: binding. The Type filter uses the eight types and No type.
- ADR-0013 An issue is placed in a worklane and a release under the repository's lock: binding. Board, List and the next issue read placement from the issue row, and this change writes no placement.

## New Durable ADRs Created

- `adr/0014-a-worklanes-issues-work-in-its-worktree-on-their-own-branch.md`
- `adr/0015-the-database-keeps-an-issues-done-date.md`
