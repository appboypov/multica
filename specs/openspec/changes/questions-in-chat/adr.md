## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0017: A chat turn may ask by the workspace default | new, supersedes ADR-0005 | `adr/0017-a-chat-turn-may-ask-by-the-workspace-default.md` |
| ADR | ADR-0018: A wake continues where the question was asked | new, supersedes ADR-0003 | `adr/0018-a-wake-continues-where-the-question-was-asked.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-15

## Review Summary

Two in-force ADRs no longer hold once agents ask in chats, and Brian's answers on WORK-447 settle both: ADR-0005 turns off asking for every task without an issue, and ADR-0003 wakes only on the link's issue with "unseen" anchored on the issue. Each is superseded by a new ADR that restates the whole decision. ADR-0018 also records the Wake checkbox and the ask-back wake of WORK-429 (pull request 34, the base of this branch), which ADR-0003 does not allow either.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: kept; no runtime machine files are read.
- ADR-0002 Questions are their own aggregate with typed answers and links: kept; a chat is one more `question_link` type, and cancelled one more status and event kind of the same tables.
- ADR-0003 Only Wake turns answers into a run: superseded by ADR-0018.
- ADR-0004 A question set groups questions; open is per question and per answerer: kept; "the last question still open for him" and a cancel ending the question for every answerer follow it.
- ADR-0005 Whether a run may ask is decided once, at claim: superseded by ADR-0017.
- ADR-0006 Questions are listed per viewer through one row list: kept; chat questions are rows of the same list, with a chat link instead of an issue.
- ADR-0007 A question reaches its parents at read time: kept; a chat link has no parents, and an `--on` link keeps its own.
- The other ADRs, also those that share the numbers 0004 to 0007 with question ADRs (organisations, project status and dates, repositories, worktrees, worklanes, releases, issue templates, done date, overview pages): kept; this change touches none of them.

The numbers 0004 to 0007 are each used by two ADRs, so a bare "ADR-0005" is ambiguous; ADR-0017 and ADR-0018 name the file they supersede beside its number.

## New Durable ADRs Created

- `adr/0017-a-chat-turn-may-ask-by-the-workspace-default.md`
- `adr/0018-a-wake-continues-where-the-question-was-asked.md`
