# ADR-0022: Each Go test package runs on its own database

- Status: accepted
- Date: 2026-10-17
- Supersedes: none

## Context

The full Go suite runs its packages side by side. When they share one database, a test can read rows another package wrote and wait on locks another package holds, so it passes alone and fails in the suite.

## Decision

`scripts/test-go.sh` migrates a fresh template database for each run and starts every package binary of the regular half through `server/internal/testutil/pkgdb`, which gives the binary its own clone of the template as `DATABASE_URL` and drops it afterwards. Tests within one package still share that package's database.

## Consequences

A test may query across workspaces and see only its own package's rows. A DB-backed test reaches its database through `DATABASE_URL`. A test run through plain `go test` uses the database `DATABASE_URL` names, shared with whatever else uses it.
