# ADR-0021: Client memberships hold organisation sets

- Status: accepted
- Date: 2026-10-16
- Supersedes: none

## Context

A client contact works for one or more organisations in one workspace. Client reads and staff management consume the same membership contract. ADR-0017's default-deny and one-membership account rules remain in force.

## Decision

Store row-owned organisation sets as non-null UUID arrays on memberships and invitations, with empty arrays for staff. Validate and deduplicate same-workspace organisations on writes. Project existing organisations on reads and copy only survivors on invitation acceptance. Do not add foreign keys or a second binding representation.

The API uses `organisation_ids` on member/invitation writes and responses and `organisations` with id/name entries on current-membership discovery. The existing member PATCH replaces the complete non-empty client set. The invitation CLI accepts repeated `--organisation` flags.

## Consequences

Small sets change atomically with their row without extra joins for persistence or secondary indexes. Organisation deletion can leave soft references, so validity projections intersect with same-workspace organisation rows. The client gate remains default deny. A downgrade to a single UUID loses all but one organisation.
