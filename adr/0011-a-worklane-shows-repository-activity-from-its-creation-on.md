# ADR-0011: A worklane shows repository activity from its creation on

- Status: accepted
- Date: 2026-10-13
- Supersedes: none (amends the read in ADR-0008)

## Context

ADR-0008 keeps a worklane's activity in `activity_log`. Under it, a repository-wide entry is one row with `repository_id` set and `worklane_id` NULL, and every worklane of the repository shows it. UJ-4 writes such entries for each plan, edit, move and removal of a release, and UJ-8 writes them when a release ships. The UJ-4 spec says that a worklane created later shows its own creation and no entry from before that, while its Timeline still shows the whole line.

## Decision

- A worklane's activity holds its own rows, plus its repository's rows created at or after the worklane's `created_at`.
- `main` is created together with its repository, so it shows every repository-wide entry.

## Consequences

- `ListActivitiesForWorklane` takes the worklane's `created_at` as a bound on the repository part. The partial index `(repository_id, created_at DESC, id DESC)` from UJ-3 serves the bound.
- A worklane's activity is the history of that worklane, not of its repository. Each new worklane does not repeat older release changes.
