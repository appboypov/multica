## At a glance

An issue agent runs `multica repo checkout owner/name` and receives its already prepared checkout path.

## Why

The command sends a task's checkout to a helper that accepts only linked worktrees. Success means the command prints the existing checkout path without creating a worktree.

## What Changes

- Reuse the prepared checkout for issue tasks marked as working in their checkout.
- Keep existing worktree preparation for other issue tasks.

## Capabilities

### Modified Capabilities

- `repository-work-in-checkout`: agents can request their task's checkout through the CLI.

## Impact

The daemon's repository checkout handler and its regression tests change. The CLI response format stays unchanged.

## Constraints

- Preserve ADR-0031's checkout routing and task path lock.
- Preserve the active task's branch and local edits.

## Non-goals

No change to repository settings or task preparation. No change to ordinary worktree behavior.

## Sources

- WORK-1329 and Brian's approved fix relayed in its handoff comment.
- `adr/0031-a-repository-can-work-only-in-its-checkout.md`
