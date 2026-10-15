## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0017: A to-do is a row the server opens and closes | new | `adr/0017-a-to-do-is-a-row-the-server-opens-and-closes.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-15

## Review Summary

ADR-0017 binds more than this change: every later to-do source opens and closes its rows through `internal/todo`, and every client reads the Inbox number from open rows.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: not touched.
- ADR-0002 Questions are their own aggregate with typed answers and links: binding. A question to-do reads the question, its answerers and its issue link, and writes none of them.
- ADR-0003 Only Wake turns answers into a run: binding. Answering a question closes its to-do and starts no run.
- ADR-0004 Organisations hold projects, and an organisation's key names its worktree folder: not touched.
- ADR-0004 A question set groups questions; open is per question and per answerer: binding. A question to-do is per question and per answerer, and closes once that answerer answered or the question is no longer open.
- ADR-0005 Project status and dates stay stored, and the web and desktop apps no longer use them: not touched.
- ADR-0005 Whether a run may ask is decided once, at claim: not touched.
- ADR-0006 Questions are listed per viewer through one server-side row list: not touched. The Questions page keeps its list; the to-do list is its own.
- ADR-0006 A repository belongs to a project and holds its worklanes: not touched.
- ADR-0007 A question reaches its parents at read time: not touched. A question to-do points at the question's own issue link.
- ADR-0007 Worktrees are made by the daemon and recorded by the server: not touched.
- ADR-0008 Worklane activity is kept in the activity log: not touched.
- ADR-0009 The server keeps worklane and issue worktree folders apart: not touched.
- ADR-0010 The server keeps each repository's release line: not touched.
- ADR-0011 A worklane shows repository activity from its creation on: not touched.
- ADR-0012 The server holds the issue templates, and a type is set once: not touched.
- ADR-0013 An issue is placed in a worklane and a release under the repository's lock: not touched.
- ADR-0014 A worklane's issues work in its worktree on their own branch: not touched.
- ADR-0015 The database keeps an issue's done date: not touched. To-dos read issue status, not `done_at`.
- ADR-0016 The Worklanes and Releases pages are built from the workspace's lists: not touched.
