## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0012: The server holds the issue templates, and a type is set once | new | `adr/0012-the-server-holds-the-issue-templates-and-a-type-is-set-once.md` |
| ADR | ADR-0013: An issue is placed in a worklane and a release under the repository's lock | new | `adr/0013-an-issue-is-placed-in-a-worklane-and-a-release-under-the-repositorys-lock.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. Both new ADRs bind more than this change:

- ADR-0012 fixes where the eight templates live, how their Markdown is rendered in two languages, which template text is translated, and that a type is set once. UJ-6 filters by type and opens New issue from a Board column, and UJ-8 lists a release's issues by type.
- ADR-0013 fixes how an issue points at its worklane and release, which lock every placement write takes, and that placement has its own endpoints apart from the issue `PATCH`. UJ-6 reads a worklane's issues and its next issue, UJ-7 reads progress per release, and UJ-8 ships a release with its issues under the same lock.

Numbering continues after ADR-0011, which lives on the UJ-4 branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: not touched. No placement or template write reads a runtime machine.
- ADR-0002 Questions are their own aggregate with typed answers and links: not touched.
- ADR-0003 Only Wake turns answers into a run: not touched.
- ADR-0004 Organisations hold projects, and an organisation's key names its worktree folder: not touched.
- ADR-0005 Project status and dates stay stored, and the web and desktop apps no longer use them: not touched. Issue dates stay as Multica has them.
- ADR-0006 A repository belongs to a project and holds its worklanes: binding. An issue in a worklane belongs to the worklane's repository's project, and repository and project deletes clear the placement of their issues.
- ADR-0007 Worktrees are made by the daemon and recorded by the server: binding. Remove worklane keeps its daemon-run remove request, and the server re-checks open issues when the daemon reports the removal.
- ADR-0008 Worklane activity is kept in the activity log: binding. Issue entries are rows with `issue_id` set, rendered on the issue page, apart from the worklane's rows.
- ADR-0009 The server keeps worklane and issue worktree folders apart: not touched. The worktree an agent uses for an issue in a worklane comes with UJ-6.
- ADR-0010 The server keeps each repository's release line: binding. Issues point at the release row, so moves and renumbering never write issues. Plan release and Remove release write their issues inside the line's transaction under its repository lock.
- ADR-0011 A worklane shows repository activity from its creation on: not touched. Issue entries are not repository-wide.

## New Durable ADRs Created

- `adr/0012-the-server-holds-the-issue-templates-and-a-type-is-set-once.md`
- `adr/0013-an-issue-is-placed-in-a-worklane-and-a-release-under-the-repositorys-lock.md`
