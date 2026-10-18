## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| Review | Held checkout ADR review | new | `specs/openspec/changes/retain-held-pull-request-checkout/adr.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-18

## Review Summary

ADR review completed for this change. Reviewed every repository ADR and its supersession links. The highest sequence in use is 0031. Duplicate sequence numbers are distinguished by filename.

The fix applies the existing checkout routing contract to the selected open pull request branch. It introduces no new storage boundary or daemon protocol. No major durable architectural decisions were introduced. No new repository-level ADR files were created. Accepted ADRs remain unchanged.

## In-Force ADRs Reviewed

Relevant context:

- `adr/0001-runtime-machine-files-are-read-by-the-daemon.md`: the daemon owns local disk operations.
- `adr/0006-repositories-hold-worklanes.md`: supersedes the organisation/worktree-path ADR-0004. Main uses the repository's recorded checkout and machine.
- `adr/0007-worktrees-are-made-by-the-daemon-and-recorded-by-the-server.md`: the server records destinations. The daemon owns worktree operations and path locks.
- `adr/0009-the-server-keeps-worklane-and-issue-worktree-folders-apart.md`: nonmatching checkout claims keep existing worktree routing.
- `adr/0014-a-worklanes-issues-work-in-its-worktree-on-their-own-branch.md`: lane location gates and branch safety remain in force.
- `adr/0031-a-repository-can-work-only-in-its-checkout.md`: amends ADR-0014 without superseding it. `InCheckout` selects the checkout path and prevents issue/PR worktree creation.

## New Durable ADRs Created

- None. No major durable architectural decisions were introduced and no repository-level ADR files were created.
