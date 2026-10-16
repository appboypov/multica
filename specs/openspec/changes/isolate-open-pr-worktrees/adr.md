# ADR Review Manifest

- Status: completed
- Review date: 2026-10-16

## At a glance

```mermaid
flowchart LR
  Claim[Server passes PR identity] --> Disk[Daemon owns worktree preparation]
  Disk --> Lock[One task per selected path]
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Review | Worktree architecture applicability | new | specs/openspec/changes/isolate-open-pr-worktrees/adr.md |

## Review Summary

ADR review completed for this change. It completes the existing PR-worktree isolation path with first-run preparation; it does not introduce another ownership model, locking model or storage system.

The ADR folder's highest number is 0021. The repository/organisation ADR-0004 is superseded by repository ADR-0006, and question-wake ADR-0003 by ADR-0020. Duplicate numeric prefixes are distinguished by filename and subject. Neither superseded record supplies a constraint for this fix. ADR-0011 amends ADR-0008's activity time boundary; this change does not alter that read.

## In-Force ADRs Reviewed

- `adr/0001-runtime-machine-files-are-read-by-the-daemon.md`: only the daemon touches runtime files.
- `adr/0006-repositories-hold-worklanes.md`: repository/runtime identity, path layout and recorded-path reuse.
- `adr/0007-worktrees-are-made-by-the-daemon-and-recorded-by-the-server.md`: preparation and path locking in the daemon, recording and safe cleanup through the existing server contract.
- `adr/0008-worklane-activity-is-kept-in-the-activity-log.md`, with `adr/0011-a-worklane-shows-repository-activity-from-its-creation-on.md`: PR preparation emits no lane switch or activity.
- `adr/0009-the-server-keeps-worklane-and-issue-worktree-folders-apart.md`: PR destinations remain separate from lane folders.
- `adr/0014-a-worklanes-issues-work-in-its-worktree-on-their-own-branch.md`: branch-owned PR worktrees and independent task path locks. The plan automates creation of the already-described registered PR tree and preserves ordinary lane flow.

## New Durable ADRs Created

None. No major durable architectural decisions were introduced and no new repository-level ADR files were created. Existing accepted ADR files remain unchanged.
