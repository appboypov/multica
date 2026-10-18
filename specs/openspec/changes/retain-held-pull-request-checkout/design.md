## At a glance

Local held checkout candidate → selected open pull request branch → matching checkout or the issue's existing worklane.

## Decision

`claimWorklane` retains a main-worklane candidate with a nonempty recorded branch only on the checkout's machine. The existing pull request selection determines whether that candidate belongs to the issue. A matching branch receives `InCheckout` and the main worklane's checkout path. A nonmatching branch uses the issue's worklane with its existing location checks. An issue without a worklane gets no block for a nonmatching branch.

Reuse the existing linked-pull-request query and its workspace/repository filters. Load failure requeues the task. Machine gates remain before pull request reads for candidates without an eligible local checkout. The selected worklane is checked before its claim block is emitted.

## Verification

`TestClaimTask_CheckoutHoldingThePullRequestBranch` failed before the fix for both an issue without a worklane and an issue in another worklane. Both pass after the fix. Different branches and closed or unrelated pull requests keep their existing routes.

The focused worklane claim tests and daemon checkout execution tests pass. A disposable HTTP claim program with PostgreSQL and a real git repository retains the checkout on `review/actual-head` with the setting off. Handler vet and the server build pass in the isolated local ARM64 container.
