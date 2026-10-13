## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0017: A release ships from a server job that redoes only what GitHub lacks | new | `adr/0017-a-release-ships-from-a-server-job-that-redoes-only-what-github-lacks.md` |
| ADR | ADR-0018: Multica writes to GitHub through the installation of the repository's owner | new | `adr/0018-multica-writes-to-github-through-the-installation-of-the-repositorys-owner.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. Both new ADRs bind more than this change:

- ADR-0017 fixes that one scheduled job does all shipping and GitHub release syncing, under the repository's lock, and that every GitHub step is skipped when GitHub already has it. Any later change that ships or edits releases goes through that job.
- ADR-0018 fixes that every write to GitHub goes through the App's installation of the repository's owner, with Contents Read and write and one token cache. Any later GitHub write reuses the client and the lookup.

Numbering continues after ADR-0016, which lives on the UJ-7 branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: not touched.
- ADR-0002 Questions are their own aggregate with typed answers and links: not touched.
- ADR-0003 Only Wake turns answers into a run: not touched.
- ADR-0004 Organisations hold projects, and an organisation's key names its worktree folder: not touched.
- ADR-0005 Project status and dates stay stored but unseen: not touched.
- ADR-0006 A repository belongs to a project and holds its worklanes: binding. Shipping reads the repository's `full_name`, default branch and version policy.
- ADR-0007 Worktrees are made by the daemon and recorded by the server: not touched. Shipping writes through GitHub, never in a checkout.
- ADR-0008 Worklane activity is kept in the activity log: binding. The ship entry is a repository-wide `activity_log` row.
- ADR-0009 The server keeps worklane and issue worktree folders apart: not touched.
- ADR-0010 The server keeps each repository's release line: binding. Shipping takes the line's lock, the next release is the line's first unshipped one, and the line's rules gain the shipped refusals in Go and TypeScript from the shared cases.
- ADR-0011 A worklane shows repository activity from its creation on: binding. Every worklane shows the ship entry through it.
- ADR-0012 The server holds the issue templates, and a type is set once: binding. The note from issues names each issue's type with the template's English name.
- ADR-0013 An issue is placed in a worklane and a release under the repository's lock: binding. Its refusal of a shipped release and its leave from one now apply to real shipped releases.
- ADR-0014 A worklane's issues work in its worktree on their own branch: not touched.
- ADR-0015 The database keeps an issue's done date: not touched. The ship date is the release's own `shipped_at`, not its issues' done dates.
- ADR-0016 The Worklanes and Releases pages are built from the workspace's lists: binding. `releaseOverview` widens to the shipped releases as it foresaw, and the counts follow the rows it keeps.

## New Durable ADRs Created

- `adr/0017-a-release-ships-from-a-server-job-that-redoes-only-what-github-lacks.md`
- `adr/0018-multica-writes-to-github-through-the-installation-of-the-repositorys-owner.md`
