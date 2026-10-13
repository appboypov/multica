# ADR-0018: Multica writes to GitHub through the installation of the repository's owner

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

Until UJ-8 the GitHub App "Pew Pew Multica" only read: pull request snapshots, the repositories an installation sees, webhooks. Shipping a release writes a commit, a tag and a release, and edits that release later. A repository row keeps `full_name` (`owner/name`) but no installation. A workspace can have several installations, one per GitHub account, and two places already mint installation tokens: `ghsnapshot.Client`, with a token cache, and the repository browse path in `handler/github.go`, without one.

## Decision

- Every write to GitHub goes through the GitHub App, with an installation token of the installation whose `account_login` is the repository's owner in the repository's workspace (`GetGitHubInstallationByAccountLogin`).
- The App's repository permission Contents is Read and write. The other permissions stay as they are.
- Writes live in one client, `server/internal/integrations/ghrelease`, which takes its tokens from `ghsnapshot.Client`'s cache through `InstallationToken`. It has no other way to authenticate.
- A repository without an installation of its owner cannot ship. The ship job logs it and tries again on the next run.

## Consequences

- A commit, tag or release by Multica shows on GitHub as made by the App, never by a person.
- Brian approves the new permission on each installation once, appboypov and deappspecialist, before the first ship.
- Later GitHub writes reuse the client and the lookup instead of adding a third way to mint tokens.
