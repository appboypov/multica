# ADR-0017: A release ships from a server job that redoes only what GitHub lacks

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

UJ-8 ships a repository's next release by itself once every issue in it is Done and every pull request linked to them is merged into the default branch. Shipping takes several writes on GitHub (a version commit, a tag, a release) and one write in Multica (the ship date and the activity entry). Any of them can fail halfway: GitHub can be down or rate limited, the server can restart, and an issue's Done can come from many paths (the issue page, the Board, a batch update, the CLI, an agent, a merged pull request). A pull request row in Multica has no base branch, and pull requests merged before UJ-8 will never send another webhook.

## Decision

- One scheduled job, `release_ship`, on the existing DB-backed scheduler with a 30 second cadence, does all shipping and all later GitHub release updates. No request handler or event listener ships.
- Each run asks the database which repositories have work: a next release that holds an issue, whose issues are all in a Done status and whose linked pull requests are all `merged`, in a repository that releases, or a shipped release whose GitHub release is behind its title or description (`github_synced_at` before `updated_at`).
- A repository ships inside one transaction holding the repository's lock (`LockRepositoryForLine`, ADR-0010), so the line cannot change while it ships. Inside it the job reads the next release again, confirms on GitHub that every linked pull request is merged into the repository's default branch, then does each GitHub step only when GitHub does not have it yet: no tag `v{version}` means commit the version file (unless it already holds the version) and tag; no GitHub release for the tag means create it. Only then does it set `shipped_at`, `github_synced_at`, the repository's `last_tag` and the activity entry, and commit.
- After a ship, the job checks the same repository again, so a done release after the one that shipped ships in the same run.
- A failed step rolls the transaction back and is logged with the repository, the version and the step. The next run starts again and skips what GitHub already has.

## Consequences

- Every path that makes an issue Done or merges a pull request leads to shipping without hooking into each path.
- A release ships up to 30 seconds after its last issue is done and merged.
- A run that fails after the tag leaves a tag without a ship date until a later run succeeds. It never makes a second commit or tag.
- Plan, edit, move and remove on that repository wait for a ship in progress, for at most the GitHub calls' timeouts.
- A release whose linked pull request merged into another branch waits, and the job confirms that on GitHub every run until it changes.
