## At a glance

Regression → checkout retention → scoped checks and HTTP/git smoke.

## 1. Backend

- [x] 1.1 `server/internal/handler/worklane_branch_test.go`: reproduce a held PR branch with the setting off for issues without a worklane and in another worklane. Run `TestClaimTask_CheckoutHoldingThePullRequestBranch` before the fix.
- [x] 1.2 `server/internal/handler/worklane_branch.go`: retain the eligible local checkout when its branch matches the selected open pull request. Preserve other routes and machine gates.
- [x] 1.3 Run the focused worklane claim tests and `TestWorkInCheckoutRunsInTheCheckout`. Run handler vet and build `cmd/server`. Exercise an HTTP claim with PostgreSQL and a real git checkout on the held branch. Remove the disposable smoke command.
