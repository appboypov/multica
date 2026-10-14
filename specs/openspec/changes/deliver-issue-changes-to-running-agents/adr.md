## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0019: Input reaches a running run as a receipt, and ✅ decides what the run's end hands on | new | `adr/0019-input-reaches-a-running-run-as-a-receipt.md` |
| ADR | ADR-0020: Only Wake turns answers into a run, and an answer its run marked ✅ is seen (supersedes ADR-0003) | new | `adr/0020-only-wake-turns-answers-into-a-run-and-a-checked-answer-is-seen.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-14

## Review Summary

ADR review completed for this change. Both new ADRs bind more than this change:

- ADR-0019 fixes one receipt store for every kind of input that reaches a running run, who may mark it, and that only ✅ decides what a run's end hands on. Any later kind of mid-run input follows it.
- ADR-0020 supersedes ADR-0003: Wake stays the only path from answers to a run, and "unseen" now leaves out answers a run marked ✅.

Numbering continues after ADR-0018, which lives on an open branch.

## In-Force ADRs Reviewed

- ADR-0001 Runtime machine files are read by the daemon: not touched.
- ADR-0002 Questions are their own aggregate: binding. Receipts point at `question_event` rows; the question model is unchanged.
- ADR-0003 Only Wake turns answers into a run: superseded by ADR-0020.
- ADR-0004 Organisations hold projects (superseded by ADR-0006), ADR-0004 Question sets group questions: not touched.
- ADR-0005 Project status and dates stay stored but unseen: not touched.
- ADR-0005 Whether a run may ask is decided at claim: binding. The "While you run" brief section, like the questions section, is decided from what the task negotiated at start.
- ADR-0006 Questions are listed per viewer, ADR-0006 Repositories hold worklanes: not touched.
- ADR-0007 A question reaches its parents at read time: binding. An answer reaches only the asking agent's run; parent assignees keep Wake.
- ADR-0007 to ADR-0016 on worktrees, worklanes, releases, templates and done dates: not touched.
- ADR-0008 Worklane activity is kept in the activity log: binding. Issue-change receipts point at `activity_log` rows and add no activity store.
- ADR-0017 and ADR-0018 on open branches (releases, GitHub writes): not touched.

## New Durable ADRs Created

- `adr/0019-input-reaches-a-running-run-as-a-receipt.md`
- `adr/0020-only-wake-turns-answers-into-a-run-and-a-checked-answer-is-seen.md`
