## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0007: A question reaches its parents at read time | new | `adr/0007-a-question-reaches-its-parents-at-read-time.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change. Its decision on where a question's parents come from binds every later reader of the issue tree for questions (lists, counts, Wake, mobile), so it is one repository-level record. Numbering continues after ADR-0006, which change `answer-every-open-question-in-one-sitting` (UJ-3) adds on its branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon (on `main`): this change reads no runtime machine files.
- ADR-0002 Questions are their own aggregate with typed answers and links (UJ-1, not yet in force): kept; links widen to eight types in the existing `question_link` table, no column added.
- ADR-0003 Only Wake turns answers into a run (UJ-1, not yet in force): kept; the agent assignees of links and parents are woken through the same endpoint and `EnqueueTaskForQuestionWake`, and "that agent's questions on the issue" becomes its wake scope (design.md decision 6).
- ADR-0004 A question set groups questions; open is per question and per answerer (UJ-2, not yet in force): kept; the indirect audience never becomes an answerer, and On my work and the parent lists use `open` and `partly_answered`.
- ADR-0005 Whether a run may ask is decided at claim (UJ-5, not yet in force): untouched; asking with links goes through the same ask endpoint.
- ADR-0006 Questions are listed per viewer through one server-side row list (UJ-3, not yet in force): kept; this change adds `on_my_work` and `parent` to the same endpoint and count, as ADR-0006 asks.

## New Durable ADRs Created

- `adr/0007-a-question-reaches-its-parents-at-read-time.md`
