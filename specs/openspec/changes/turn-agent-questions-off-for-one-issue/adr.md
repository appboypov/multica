## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0005: Whether a run may ask is decided once, at claim | new | `adr/0005-whether-a-run-may-ask-is-decided-at-claim.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-12

## Review Summary

ADR review completed for this change. Its decision on when a run's question permission is made binds every way to ask (UJ-1's ask, UJ-2's ask of several) and every later text about questions, so it is a repository-level record. Numbering continues after ADR-0004, which change `agent-asks-several-questions-at-once` (UJ-2) adds on its branch.

## In-Force ADRs Reviewed

- None on `main`: `adr/` does not exist there.
- ADR-0001 Runtime machine files are read by the daemon (open #9, not yet in force): kept; this change reads no runtime machine files.
- ADR-0002 Questions are their own aggregate with typed answers and links (UJ-1, not yet in force): kept; the setting is on the issue, the workspace and the task, never on `question`.
- ADR-0003 Only Wake turns answers into a run (UJ-1, not yet in force): kept; Wake works while questions are off, and the woken run gets its answers without the tool's line.
- ADR-0004 A question set groups questions; open is per question and per answerer (UJ-2, not yet in force): kept; a refused ask of several writes none of them, as one ask is one transaction.

## New Durable ADRs Created

- `adr/0005-whether-a-run-may-ask-is-decided-at-claim.md`
