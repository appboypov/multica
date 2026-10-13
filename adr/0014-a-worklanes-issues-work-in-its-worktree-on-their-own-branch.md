# ADR-0014: A worklane's issues work in its worktree on their own branch

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

UJ-2 gives an issue a worktree per repository at `~/Worktrees/{organisation key}/{repo}/{issue key}` that lasts as long as the issue. UJ-3 gives each worklane its own long-lived worktree, made detached at `origin/{default_branch}`, and worklane `main` works in the repository's checkout. UJ-5 places issues in worklanes. UJ-6 makes a worklane the place its issues are worked: one at a time, in their order, on the worklane's machine, with Busy or Free and now or next shown on the worklane and, in UJ-7, on every worklane row. Brian wants no exception logic for issues that do not fit a worklane.

## Decision

- An agent's task on an issue in a worklane runs in that worklane's worktree, or in the repository's checkout for `main`, on the issue's branch: the issue key in lowercase, cut from the fetched `origin/{default_branch}` when it does not exist. Such an issue gets no issue worktree. Issues without a worklane keep UJ-2's issue worktree.
- A worklane runs one task at a time: the daemon's path lock on the worktree serialises its tasks.
- A task whose agent's runtime is on another machine than the worklane's fails at claim with `worklane_elsewhere`, naming both machines.
- The worktree changes branch only when it holds nothing uncommitted and no commit missing from its remote. Otherwise a task fails with `worklane_branch_held`, and a switch after a merge keeps the branch and says why in the worklane's activity.
- When the pull request on the worklane's branch merges into the default branch, the server asks the daemon to switch the worktree to the worklane's next issue's branch, or to detach it at `origin/{default_branch}` when none is left.
- The daemon reports every branch it switches to, and the server keeps it in `worklane.branch`. Busy, the active issue and the next issue are computed from that branch and the worklane's open issues on every read, never stored.

## Consequences

- One worktree per worklane holds all its work, so an agent never works a worklane's issue elsewhere.
- Busy reflects what the worktree actually holds: an issue moved to In Progress without a task leaves the worklane Free.
- A switch request lost with a machine or a server restart costs nothing: the next task switches anyway.
- An issue moved between worklanes while its branch is checked out in the first one fails its next task with git's message until the first worklane leaves the branch.
- UJ-7 reads now and next from the worklane response without its own rule.
