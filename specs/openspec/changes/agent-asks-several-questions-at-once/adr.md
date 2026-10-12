## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0004: A question set groups questions; open is per question and per answerer | new | `adr/0004-question-sets-group-questions-open-is-per-question.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-12

## Review Summary

ADR review completed for this change. Its decisions on how a set is stored and what "open" means bind UJ-3 and UJ-4 too, so they are one repository-level record. Numbering continues after ADR-0003, which change `answer-an-agents-question-and-wake-it` (UJ-1) adds on its branch.

## In-Force ADRs Reviewed

- None on `main`: `adr/` does not exist there.
- ADR-0001 Runtime machine files are read by the daemon (open #9, not yet in force): this change reads no runtime machine files.
- ADR-0002 Questions are their own aggregate with typed answers and links (UJ-1, not yet in force): kept; a set is columns on `question`, not a new table, so its five tables stay as they are.
- ADR-0003 Only Wake turns answers into a run (UJ-1, not yet in force): kept; the set's footer calls the same Wake endpoint, and its guard keeps the one unseen query.

## New Durable ADRs Created

- `adr/0004-question-sets-group-questions-open-is-per-question.md`
