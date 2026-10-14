## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0006: Questions are listed per viewer through one server-side row list | new | `adr/0006-questions-are-listed-per-viewer-through-one-row-list.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-12

## Review Summary

ADR review completed for this change. Its decision on where the per-viewer lists and the count are decided binds UJ-4 too, which adds the On my work tab, parents and the indirect audience to the same list, so it is one repository-level record. Numbering continues after ADR-0005, which change `turn-agent-questions-off-for-one-issue` (UJ-5) adds on its branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon (on `main`): this change reads no runtime machine files.
- ADR-0002 Questions are their own aggregate with typed answers and links (UJ-1, not yet in force): kept; the lists read the five question tables and add only indexes.
- ADR-0003 Only Wake turns answers into a run (UJ-1, not yet in force): kept; Answer all sends each answer on its own and the Wake bar calls UJ-1's Wake endpoint, so no answer starts a run.
- ADR-0004 A question set groups questions; open is per question and per answerer (UJ-2, not yet in force): kept; the For me tab and the count apply its rule in SQL, with a parity test against `question.IsOpenFor`, and a set's questions stay together by `ask_position`.
- ADR-0005 Whether a run may ask is decided once, at claim (UJ-5, not yet in force): untouched; this change lists and answers questions already asked.

## New Durable ADRs Created

- `adr/0006-questions-are-listed-per-viewer-through-one-row-list.md`
