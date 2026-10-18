## At a glance

A task stays in the checkout when it holds the issue's open pull request branch after Work in the project folder is turned off.

## Why

`claimWorklane` recognises the issue-key branch but not a differently named pull request branch. Losing `InCheckout` sends the daemon to a pull-request worktree that git cannot create while the checkout holds that branch.

## What Changes

- Retain the checkout for the selected linked open pull request in the same workspace and repository.
- Preserve the existing worklane route when its branch does not match.
- Keep runtime-machine authorization and claim-failure redelivery.

## Capabilities

### Modified Capabilities

- `repository-work-in-checkout`: retain a held open pull request branch with the setting off.

## Impact

`server/internal/handler/worklane_branch.go` and its adjacent claim tests. No API shape or database change. Daemon preparation uses its existing `InCheckout` path.

## Sources

WORK-1330 and Brian's approval recorded on its issue. ADR-0031 defines checkout execution.
