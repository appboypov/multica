## At a glance

```mermaid
flowchart LR
  Claim[1. API claim context] --> Routing[2. Daemon PR worktree routing]
```

| Kind | Name | Change | Path |
|---|---|---|---|
| API group | Backend / madspec-api-builder | changed | server/pkg/protocol/messages.go; server/internal/handler/worklane_branch.go |
| Service group | Backend / madspec-service-builder | changed | server/internal/daemon/worklane_task.go; server/internal/daemon/issue_worktree.go |
| Git preparation | Branch-owned PR worktree | new | server/internal/daemon/execenv/pull_request_worktree.go |
| Tests | Claim selection and simultaneous tasks | changed | server/internal/handler/worklane_branch_test.go; server/internal/daemon/worklane_task_test.go |
| Docs | PR worktree selection and waits | changed | CLI_AND_DAEMON.md |
| System map | Daemon worktree flow | changed | specs/docs/system-design.md |

## 1. API claim context

- [ ] 1.1 `server/pkg/protocol/messages.go` - add optional `ClaimWorklane.pull_request_branch` for the repository's linked open PR; inspect symbol references before changing the exported contract and update affected consumers without adding another endpoint or database field.
- [ ] 1.2 `server/internal/handler/worklane_branch.go` - populate the field from `ListPullRequestsByIssue` with matching workspace/repository and open state, using its newest-first order; cover existing and appended claim repositories and route query errors through `loadFailed`, preserving the daemon gate.
- [ ] 1.3 `server/internal/handler/worklane_branch_test.go` - add fixture-backed claim tests for open versus closed/merged PRs and wrong-repository/workspace exclusion, asserting the selected branch and unchanged authorization; run the focused handler cases with the repository's isolated test setup.

## 2. Daemon worktree routing

- [ ] 2.1 `server/internal/daemon/worklane_task.go`, `server/internal/daemon/issue_worktree.go` - select recorded/registered PR trees or compute the missing PR destination before locking; use the actual PR branch, canonicalize missing paths consistently with later existing paths, keep current non-PR behavior, and acquire only the PR path lock for a PR run.
- [ ] 2.2 `server/internal/daemon/execenv/pull_request_worktree.go`, `server/internal/daemon/worklane_task.go` - prepare a missing worktree under the task and Git-root locks from the existing local branch or fetched origin PR branch, validate and reuse existing worktrees unchanged, record the resolved PR tree with `RecordIssueWorktree`, and return without reporting a lane branch switch; preserve normal Git/environment errors without falling back to the lane or forcing a shared branch checkout.
- [ ] 2.3 `server/internal/daemon/worklane_task_test.go` - extend the canonical real-Git tests for present/missing PR trees while a dirty, unpushed next-issue builder still holds the lane lock; assert verifier progress before lane unlock, exact branch/HEAD/file preservation, no lane-branch report, same-PR exclusion, remote-only branch contents and reuse of review fixes. Bound contention with deadlines/channels. Run these focused cases and exercise the selection/preparation path in a disposable real-Git smoke harness without executing installed agent CLIs; retain the observed working directories and lock ownership as build evidence and remove the harness.
- [ ] 2.4 `CLI_AND_DAEMON.md`, `specs/docs/system-design.md` - document the PR-owned directory and independent lane/PR waits beside Agent Daemon, and update the system map's affected daemon worktree node and edges to match the implementation; include first-run creation without release/recovery procedures or unrelated map changes.

## Implementation Notes

The source of truth for acceptance is `specs/worklane-agent-worktree/spec.md`. Joeri builds the two groups in dependency order using their named backend builders, runs the relevant Go checks/build, and opens the issue's single implementation PR. Planning does not release or manipulate the blocked AMD runs. The verifier owns full verification, archive and merge; strict OpenSpec validation runs again before archive under that workflow.

## Plan Change Log

- 2026-10-16: Planned WORK-1201 from its issue and Skuddy's explicit request. Added first-run PR-worktree creation and simultaneous builder/verifier acceptance. Corrected the prior root lookup: OpenSpec commands run in `specs/`.
