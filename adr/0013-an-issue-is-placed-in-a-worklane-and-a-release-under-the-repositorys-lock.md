# ADR-0013: An issue is placed in a worklane and a release under the repository's lock

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

UJ-5 puts issues in a worklane and in a release of that worklane's repository's line (ADR-0010). An issue in a worklane belongs to the worklane's project. Issues follow their release when it moves or renumbers, and go back to "No release" when it is removed. Releases are planned, moved and removed under a lock on the repository row. UJ-6 shows a worklane's issues on Board and List and picks its next issue, UJ-7 shows progress per release, and UJ-8 ships a release with its issues and refuses adding issues to a shipped release. The fork also merges upstream releases, so every change to upstream's hot issue update path costs a merge later.

## Decision

- `issue.worklane_id` and `issue.release_id` are nullable columns without foreign keys. `release_id` names the release row, never its version.
- A release needs a worklane, and must be an unshipped release of the worklane's repository when it is set. With a worklane, `issue.project_id` is the worklane's repository's project.
- The rules live in one pure Go package, `server/internal/issueplacement`. Every write that sets or clears an issue's worklane or release, including creation, Plan release, Add issue to release and Remove release, runs in one transaction that first locks the repository rows involved, in id order, with the lock the release writes take.
- Placement and type have their own endpoints, `PUT /api/issues/{id}/placement` and `PUT /api/issues/{id}/type`. The issue `PATCH` and its SQL stay as upstream has them, apart from refusing a project other than the worklane's.
- Each placement change writes an issue activity row in the same transaction, naming the release and worklane as they were.
- Removing a release, a repository or a project clears the placement of its issues in its own transaction. Removing a worklane is refused while an open issue is in it, and its closed issues keep their release under "No worklane".

## Consequences

- No issue can land in a release that is being removed, or of another repository, through a race.
- Moving and renumbering releases never write issues.
- UJ-6, UJ-7 and UJ-8 read placement from the issue row, and UJ-8's shipping takes the same lock.
- A closed issue of a removed worklane is the only issue with a release and no worklane. Any later placement write applies the rules again.
- Upstream merges touch `issue.go` only where creation and the project refusal were added.
