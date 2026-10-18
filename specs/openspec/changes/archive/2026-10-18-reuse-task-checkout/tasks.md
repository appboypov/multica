## At a glance

One daemon handler fix with real Git regression coverage and CLI smoke proof.

## 1. Regression

- [x] 1.1 Add `TestRepoCheckoutReusesTaskCheckout` in `server/internal/daemon/health_test.go` and observe failure for checkout-mode issue and PR branches.

## 2. Implementation

- [x] 2.1 Reuse the prepared checkout in `server/internal/daemon/health.go` when `Worklane.InCheckout` is true. Run checkout handler and task-routing tests.

## 3. Verification

- [x] 3.1 Smoke the built CLI against the real checkout handler. Run Go vet and build for the daemon and CLI.
- [x] 3.2 Run `openspec validate reuse-task-checkout --type change --strict`.

## Verification evidence

The regression failed before the fix with HTTP 500 for both branches. The checkout was rejected as not being a linked worktree.

The fixed tree passed `go test ./internal/daemon -run "^Test(RepoCheckout|WorkInCheckout)" -count=1 -v`. A temporary harness ran the built `multica repo checkout test/repo` against the authenticated daemon handler. Its stdout was the checkout path and its worktree list stayed unchanged. The harness is removed after the smoke.

`go build -o /tmp/multica-checkout-smoke ./cmd/multica` and `go vet ./internal/daemon ./cmd/multica` passed. `gofmt -l` reported no changed Go files.

Execution used Crabbox local-container on Linux ARM64 with Go 1.26.6. Image: `sha256:067a5f41a870e45a2f707913b0141a7cc67c71878bbeb7bd57d90c16c39e55fb`. Resources: 2 CPUs and 3 GiB RAM. The fork requires local checks. Lease `cbx_159682a75dd4` exited 0 with container removal confirmed.

The full repository check belongs to verifier review. Production installation is unchanged.

## Review correction

- [x] Guard `RecordIssueWorktree` with checkout ownership in `server/internal/daemon/health.go`.
- [x] Reject checkout cleanup registration for both branch scenarios. Preserve registration for ordinary issue and worklane worktrees in `server/internal/daemon/health_test.go`.
- [x] Update `design.md` to keep the checkout outside issue-worktree cleanup.

Sarkout's `TestVerifierNoIssueWorktreeRecord` supplied the failing-before evidence. That failed check was not rerun to confirm the report.

The corrected tree passed the checkout and task-routing test command above. The built CLI smoke passed on both the issue and PR branches with no issue-worktree registration. Ordinary worktree registration passed with and without a worklane. Go formatting and build and vet passed using the commands above. The temporary smoke harness is removed.

The correction used the same isolated local Linux ARM64 image and Go runtime. Lease `cbx_d9f277ce656c` exited 0 with container removal confirmed.
