# ADR-0008: Worklane activity is kept in the activity log

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

A worklane's page shows its activity: who did what and when. UJ-3 adds creating and renaming a worklane. UJ-4 adds planning, editing, moving and removing releases, which show on every worklane of the repository. UJ-6 adds issues and Busy, and UJ-8 adds shipping. Multica already keeps an issue's history in `activity_log` (`workspace_id`, a nullable `issue_id`, actor, `action`, `details`), read by issue with keyset order, and deleted with the workspace.

## Decision

- A worklane's entries are `activity_log` rows with `worklane_id` set. An entry for every worklane of a repository is one row with `repository_id` set and `worklane_id` NULL. Neither column is a foreign key.
- A worklane's activity is its own rows plus its repository's rows, oldest first, capped at the newest 200, read through partial indexes on each column.
- An entry keeps what it shows at the time it happened in `details`, such as the machine's name, so later renames do not rewrite history.
- The client ignores actions it does not know, so newer servers can add actions for older desktop apps.
- Deleting a worklane, repository or project deletes its entries in the same transaction.

## Consequences

- One table, one actor model and one rendering path serve issues and worklanes; no second activity table.
- UJ-4 to UJ-8 add actions and write rows; the read, the indexes and the page do not change.
- Existing reads of `activity_log` filter by `issue_id` or `action` and are unaffected; their rows have both new columns NULL.
