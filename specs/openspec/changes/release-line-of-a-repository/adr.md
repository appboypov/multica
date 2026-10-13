## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0010: The server keeps each repository's release line | new | `adr/0010-the-server-keeps-each-repositorys-release-line.md` |
| ADR | ADR-0011: A worklane shows repository activity from its creation on | new | `adr/0011-a-worklane-shows-repository-activity-from-its-creation-on.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. Both new ADRs bind more than this change:

- ADR-0010 fixes where a release line lives, how versions renumber, and how the line stays strictly increasing under concurrent writes. UJ-5 points issues at a release, UJ-7 lists every line, and UJ-8 ships the first unshipped release and fixes its version.
- ADR-0011 fixes which repository-wide entries a worklane shows. It binds UJ-8's shipping entries and every later repository-wide action.

Numbering continues after ADR-0009, which lives on the UJ-3 branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: binding. The last tag comes from the checkout check the daemon runs; the server never reads a checkout.
- ADR-0002 Questions are their own aggregate with typed answers and links: not touched.
- ADR-0003 Only Wake turns answers into a run: not touched.
- ADR-0004 Organisations hold projects, and an organisation's key names its worktree folder: not touched.
- ADR-0005 Project status and dates stay stored, and the web and desktop apps no longer use them: binding. Projects and an organisation's page gain a release column and show no project status or dates.
- ADR-0006 A repository belongs to a project and holds its worklanes: binding. The release line belongs to the repository, and every worklane of the repository shows it.
- ADR-0007 Worktrees are made by the daemon and recorded by the server: not touched. No release action touches a worktree.
- ADR-0008 Worklane activity is kept in the activity log: binding. Release changes are repository-wide rows with `repository_id` set and `worklane_id` NULL. ADR-0011 bounds which of them a worklane shows.
- ADR-0009 The server keeps worklane and issue worktree folders apart: not touched.

## New Durable ADRs Created

- `adr/0010-the-server-keeps-each-repositorys-release-line.md`
- `adr/0011-a-worklane-shows-repository-activity-from-its-creation-on.md`
