# ADR-0015: The database keeps an issue's done date

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

UJ-6 records when an issue reaches Done and drops the date when it leaves Done. UJ-8 derives a release's ship date from it. Multica writes an issue's status from many queries: the issue update, the batch update, PR auto-complete and others, and the fork merges upstream releases that add more. A custom status counts as Done through its category in the workspace's status catalogue, which `issue_effective_status` resolves in SQL.

## Decision

- `issue.done_at` is set by the trigger `trg_issue_done_at`, `BEFORE INSERT OR UPDATE OF status ON issue`: `now()` when the status becomes a done status, kept when it moves between done statuses, NULL otherwise.
- No Go code writes `done_at`. A catalogue change to a status's category does not rewrite dates.
- Issues already done when the column arrives get the time of their latest `status_changed` entry to their status, else `updated_at`.

## Consequences

- Every status writer, upstream's later ones included, keeps the date without a change of its own.
- The trigger is the one place outside Go that writes an issue field, and it is named here so it is not missed.
- Dates of issues done before the column are as good as their activity entries.
