## At a glance

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  Make[make test] --> Runner[scripts/test-go.sh]:::changed
  Runner -- "pkgdb create" --> Template[(fresh template DB)]:::new
  Runner -- "cmd/migrate up" --> Template
  Runner -- "go test -exec 'pkgdb run'" --> PkgDB[pkgdb run]:::new
  PkgDB -- "CREATE DATABASE … TEMPLATE" --> Clone[(one DB per package binary)]:::new
  PkgDB -- "DATABASE_URL = clone" --> Binary[package test binary]
  Binary --> Clone
  Runner -- "pkgdb drop, on every exit" --> Template
  Agent[pkg/agent half]
  Runner --> Agent
```

```mermaid
sequenceDiagram
  participant R as test-go.sh
  participant P as pkgdb
  participant G as go test
  participant PG as worktree Postgres
  R->>P: create
  P->>PG: CREATE DATABASE multica_test_<hex>
  R->>PG: cmd/migrate up on the template
  R->>G: test -race -exec "pkgdb run" <regular packages>
  loop each package binary, in parallel
    G->>P: run <binary> <args>
    P->>PG: CREATE DATABASE <template>_<hex> TEMPLATE <template>
    P->>P: start binary with DATABASE_URL = clone
    P->>PG: DROP DATABASE clone WITH (FORCE)
    P-->>G: binary's exit status
  end
  R->>P: drop (trap on EXIT, INT, TERM)
  P->>PG: DROP the template and every clone named after it
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Test tool | `pkgdb`: template database and per-package clones | new | `server/internal/testutil/pkgdb/main.go` |
| Test | `pkgdb` clone, exit status and cleanup | new | `server/internal/testutil/pkgdb/main_test.go` |
| Runner | Go suite runner, regular half on per-package databases | changed | `scripts/test-go.sh` |
| Runner test | Runner call order and cleanup | changed | `scripts/test-go.test.sh` |
| Docs | Go test database note | changed | `CONTRIBUTING.md` |

No product part changes. The living system design, `specs/docs/system-design.md`, maps the product and not its test tooling, so this change has no delta on it; the repository has no `prototypes/system/`.

## Context

### Reproduced failures and their causes

**`TestListStaleUndecidedGitHubPRsExcludesDecidedAndRotatesCursor` (`internal/integrations/ghsnapshot`).** Reproduced deterministically: with the row that `seedDupRacePR` (`server/internal/handler/comment_duplicate_enqueue_race_test.go:682-700`) writes present in the database, the test fails exactly as in the full suite: `refresh_db_test.go:192: first bounded sweep = [{InstallationID:1 RepoOwner:multica-ai RepoName:multica PrNumber:999320}], want first address`. Cause: `ListStaleUndecidedGitHubPRs` (`server/pkg/db/queries/github_snapshot.sql:46`) sweeps every workspace by design, and the test asserts the sweep's first address. A handler test's open PR row that is in the database at that moment comes first.

**`TestTurnedOffChildDoneRuleLeavesARequeuedRun` (`internal/service`).** The full-check run on PR #80 (Sarkout's run `01a14772-fbf1-7b34-9d89-029418dd4857`, capture `run_bb6b27aab95c499607d15ee78eace14e`) failed at `issue_wakeup_join_test.go:147: the first claim got nothing`. The `issue` subtest took 14.40 s, the test 63.10 s and the `service` package 121 s; the same test takes about 0.04 s alone. Five later full-suite runs and 240 in-suite repetitions on a local container did not reproduce it, so it depends on how slow the shared database is at that moment.

The failing path, proven with throwaway tests in this worktree:

1. The second `ProcessChildEvents` (line 143) dispatches the child-done rule through `dispatchSystem` (`server/internal/service/issue_wakeup_system.go:388`), which has a 2 s deadline and `lock_timeout = 50ms` and locks the rule row (`LockIssueWakeup`, line 429).
2. When the deadline ends the dispatch part way, `ProcessChildEvents` returns nil: a failed dispatch is left to the scheduler (lines 249-251). With deadlines of 1-4 ms around that call, 7 of 120 runs returned nil and then got nothing on the first claim.
3. The claim's `JoinWaitingWakeups` takes the rule with `TryLockIssueWakeup`, `FOR UPDATE SKIP LOCKED` (`issue_wakeup_join.go:224`). Observed: while another session holds the rule row, the first claim gets nothing; with no lock, or with a `SHARE ROW EXCLUSIVE` lock on `agent_task_queue` as `CREATE TRIGGER` takes, it gets the rule's input. [INFERENCE] In the failing run, the dispatch abandoned at its deadline still held the rule row when the claim ran.

Cause: in the full suite the dispatch overruns its 2 s deadline because the database it shares with every other package stalls. Other packages lock rows and tables there that this package also uses: handler tests create and drop triggers on the shared `agent_task_queue` and `agent` tables (`daemon_runtime_access_test.go:545-603`, `workspace_delete_fence_test.go:440-463`), lock tables (`source_context_integration_test.go:680`), and run sweeps over every workspace (`issue_wakeup_v2_test.go:23,57`, `issue_system_wakeup_test.go:183`). A table lock request waits behind open transactions, and every later write to that table waits behind it. Neither the wakeup service nor the test is wrong: the test needs a database only its own package uses.

### What is shared today

- `scripts/test-go.sh` runs every package outside `pkg/agent` in one `go test` call at the default `-p`, so up to GOMAXPROCS package binaries run at once.
- 24 test packages read `DATABASE_URL` (`os.Getenv("DATABASE_URL")`, most with `postgres://multica:multica@localhost:5432/multica` as default). All of them get the one database that `make test` migrated.
- `make test` runs `scripts/ensure-postgres.sh`, `go run ./cmd/migrate up` on that database, then `scripts/test-go.sh --race`. `scripts/check.sh` migrates the same database before `test-go.sh`, and its E2E step uses it afterwards.
- `server/internal/taskusagebackfill/hook_test.go:240-277` and `server/cmd/backfill_codex_usage_cache/integration_test.go:262-274` already create and drop their own databases with `CREATE DATABASE` and `DROP DATABASE … WITH (FORCE)`; the test user can create databases.

### Measured cost

On a fresh Postgres 16 in the local container: migrating an empty database 0.45 s, the migrated database is 16 MB, one `CREATE DATABASE … TEMPLATE` 0.06 s, 16 parallel clones of the same template 0.94 s, 16 parallel drops 0.37 s. All 16 parallel clones succeeded.

### Team

One group, built by the Infra team (Tortolero), whose kinds include `pipeline` and `environment`: the Go suite runner and its database environment. One pull request.

## Goals / Non-Goals

**Goals**

- Always: every package binary of the regular half runs against its own database, cloned from a template migrated fresh for that run.
- Always: the runner drops the template and every clone it made, also when the suite fails or is interrupted.
- Always: the runner makes its databases on the server and with the credentials of `DATABASE_URL`, named `multica_test_<8 hex>` for the template and `multica_test_<8 hex>_<8 hex>` for its clones.
- Always: without a reachable database the suite runs as today and DB-backed tests skip.
- Never: the runner writes to the database `DATABASE_URL` names, or to the production stack's database.
- Never: a test loses an assertion, and the wakeup service and `ListStaleUndecidedGitHubPRs` stay as they are.

**Acceptance**

The change sets `skip_specs: true`, so its acceptance lives here. Every check runs on Crabbox, as `madspec-remote-checks-and-tests` says, never against the main checkout's `.env` stack.

- `bash scripts/test-go.test.sh` passes.
- `go test ./internal/testutil/pkgdb/` passes against a reachable Postgres.
- With the row `multica-ai/multica#999320` that `seedDupRacePR` writes inserted into the database `DATABASE_URL` names, `TestListStaleUndecidedGitHubPRsExcludesDecidedAndRotatesCursor` fails under plain `go test` and passes through `scripts/test-go.sh`.
- On `main` with this change, `bash scripts/test-go.sh --race --only regular` passes, and `SELECT datname FROM pg_database WHERE datname LIKE 'multica_test_%'` returns no row afterwards.

**Non-Goals**

- The `pkg/agent` half, its throttling and its `go test` call.
- Flaky tests that do not depend on the database, such as `internal/daemon` `TestReadTaskWakeupMessagesExtendsReadDeadlineOnPeerTraffic`, which failed on 2-3 CPU local containers during this investigation.
- Lowering package parallelism, or changing timeouts in production code.

## Decisions

### 1. One database per package binary through `go test -exec`

`go test -exec 'pkgdb run'` starts each package's test binary through `pkgdb`, which clones the template, runs the binary with `DATABASE_URL` set to the clone, drops the clone and exits with the binary's status. Every package that reads `DATABASE_URL` is isolated without per-package code, and a new DB-backed package is isolated without anyone remembering to. Later DB-backed tests rely on this, so it is recorded in `adr/0022-each-go-test-package-runs-on-its-own-database.md`.

Rejected: a `TestMain` helper in each DB-backed package (24 packages to change, and a new package that forgets it shares again); `go test -p 1` (removes running side by side, but not rows left behind, and makes the suite several times slower); per-test cleanup fixes in the two tests (the shared-state classes stay: global sweeps, table locks).

### 2. A fresh template per run

`pkgdb create` makes an empty database `multica_test_<8 hex>`, the runner migrates it with `go run ./cmd/migrate up`, and the clones are `<template>_<8 hex>`. Cloning the worktree's own migrated database instead would carry every row that earlier runs on the shared database left behind into each clone. The names have a fixed length of 21 and 30 bytes: worktree database names, `multica_<worktree slug>_<n>` from `scripts/init-worktree-env.sh`, have no length limit, and a name built on them could pass Postgres's 63-byte limit, which truncates names silently.

`pkgdb` connects to the `postgres` maintenance database of the same server for `CREATE` and `DROP`, so nothing is connected to the template while it is cloned. `pkgdb drop` removes the template and every database whose name starts with the template's name and `_`, so a clone of a killed binary goes with it.

### 3. `pkgdb` is a Go program under `internal/testutil`

`server/internal/testutil/pkgdb` is a `main` package using `pgx`, which the server module already has, so the runner needs no `psql` on the host. `test-go.sh` builds it once per run into a temporary directory. Commands:

| Command | Does |
|---|---|
| `pkgdb create` | Prints the template's URL. Prints nothing and exits 0 when `DATABASE_URL` is unset or unreachable. |
| `pkgdb run <binary> <args…>` | With `MULTICA_TEST_TEMPLATE_URL` set: clone, run, drop, exit with the binary's status; it forwards SIGINT and SIGTERM to the binary. Without it: runs the binary unchanged. |
| `pkgdb drop <template URL>` | Drops the template and its clones with `WITH (FORCE)`. |

### 4. The worktree database stays as it is

`make test` and `scripts/check.sh` keep migrating the database `DATABASE_URL` names: `go test` run by hand and the E2E step use it. The runner only reads its server and credentials.
