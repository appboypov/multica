# ADR-0010: The server keeps each repository's release line

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

A repository has one release line, which every worklane of the repository shows. Each release has a version, a title and a description. Its version is either computed from its bump (patch, minor or major) after the release before it, or set by hand. Versions must strictly increase along the line, and the first must be higher than the repository's last tag. UJ-4 plans, edits, moves and removes releases. UJ-5 puts issues in releases, and those issues follow their release when it moves. UJ-7 lists every repository's line, and UJ-8 ships the first unshipped release and fixes its version. Several members and agents can change the same line at the same time.

## Decision

- A release is a row of `release`. It holds its `repository_id`, its `position` on the line, its `version` without the `v` (the tag is `v{version}`), its `bump` (NULL when the version was set by hand), its title and description, and `shipped_at` (NULL until it ships). There are no foreign keys.
- The repository keeps the version of the last tag (its highest `v{version}` tag) in `repository.last_tag`, without the `v`. A checkout check finds it once, at Add repository, or when a passing check meets a NULL. NULL means not known yet, and `''` means the repository has no tag. Writes to the line refuse while it is NULL.
- One Go package, `server/internal/releaseline`, holds the line's rules: renumbering, the strictly-increasing check, and plan, edit, move and remove. Every write runs in one transaction:
  1. It locks the repository row.
  2. It reads the line and applies the operation.
  3. It renumbers every computed version after the shipped releases, from the release before it, or the last tag, or 0.0.0.
  4. It refuses a line whose versions do not strictly increase.
  5. It rewrites the positions and versions that changed.
  6. It writes one repository-wide activity row that names every changed version.
- Shipped releases keep their position and version. The next release is the first unshipped one, and nothing stores it.
- Clients preview the same rules on the cached line with a TypeScript port in `packages/core/releases/`, so the dialog and menu can say why something is refused before anything is sent. The server's answer decides. Both ports run the same case file, `server/internal/releaseline/testdata/cases.json`.
- Every write answers with the repository's whole line and publishes `release_line:updated` with the repository's id.

## Consequences

- No version can break the line through a race. The repository lock serialises writes to one line, and the check runs on the line as it stands.
- Issues refer to their release by id (UJ-5), so renumbering and moving never touch issues.
- UJ-8 sets `shipped_at` and adds its refusals. The renumbering already skips shipped releases, and "next" already follows from `shipped_at`.
- The rules live in two languages. The shared case file catches any drift between them.
- A repository added before this change has no stored last tag until a passed checkout check fills it in.
