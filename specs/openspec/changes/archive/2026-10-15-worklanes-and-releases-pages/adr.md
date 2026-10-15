## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0016: The Worklanes and Releases pages are built from the workspace's lists | new | `adr/0016-the-worklanes-and-releases-pages-are-built-from-the-workspaces-lists.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. The new ADR binds more than this change:

- ADR-0016 fixes that Worklanes and Releases are built in the client from the workspace's lists in Projects' order, that a release's progress counts every issue in it from the server's per-release totals, and that the counts follow the rows kept. UJ-8 adds shipped releases to Releases on it.

Numbering continues after ADR-0015, which lives on the UJ-6 branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: not touched.
- ADR-0002 Questions are their own aggregate with typed answers and links: not touched.
- ADR-0003 Only Wake turns answers into a run: not touched.
- ADR-0004 Organisations hold projects, and an organisation's key names its worktree folder: binding. The pages group by organisation through the project.
- ADR-0005 Project status and dates stay stored, and the web and desktop apps no longer use them: binding. The pages show no project status or date.
- ADR-0006 A repository belongs to a project and holds its worklanes: binding. The tree is organisation, project, repository, worklane or release, and every repository has `main`.
- ADR-0007 Worktrees are made by the daemon and recorded by the server: not touched.
- ADR-0008 Worklane activity is kept in the activity log: not touched. The pages write nothing.
- ADR-0009 The server keeps worklane and issue worktree folders apart: not touched.
- ADR-0010 The server keeps each repository's release line: binding. Releases lists the line in position order, with next and planned from `lineStates`.
- ADR-0011 A worklane shows repository activity from its creation on: not touched.
- ADR-0012 The server holds the issue templates, and a type is set once: not touched.
- ADR-0013 An issue is placed in a worklane and a release under the repository's lock: binding. The release totals and the worklanes of a release read placement from the issue row.
- ADR-0014 A worklane's issues work in its worktree on their own branch: binding. Now or next on every worklane row is the server's `state`, `active_issue` and `next_issue`, which gain the issue's status.
- ADR-0015 The database keeps an issue's done date: not touched. UJ-8 reads it for the ship date.

## New Durable ADRs Created

- `adr/0016-the-worklanes-and-releases-pages-are-built-from-the-workspaces-lists.md`
