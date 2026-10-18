## At a glance

`repoCheckoutHandler` uses the active task's prepared checkout when its worklane has `InCheckout`. Other issue tasks keep `PrepareIssueWorktree`.

## Context

Task preparation selects the authoritative path and holds its path lock throughout execution. `PrepareIssueWorktree` deliberately rejects the checkout itself.

## Decision

Start with `repo.WorktreePath`. Call `PrepareIssueWorktree` and `RecordIssueWorktree` only when the task is not marked `InCheckout`. The checkout belongs to its worklane rather than issue-worktree cleanup. Preserve branch lookup after path selection. No public API or exported symbol changes.

## Verification

Exercise task preparation followed by the authenticated checkout handler using real temporary Git repositories. Cover an issue branch and an open PR branch. Assert that the returned path and branch match the task and that worktrees and local edits stay unchanged. Reject issue-worktree registration for checkout mode. Require registration for ordinary issue and worklane worktrees. Smoke the built CLI against the handler.
