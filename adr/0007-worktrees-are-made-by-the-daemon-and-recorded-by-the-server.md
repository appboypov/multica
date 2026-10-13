# ADR-0007: Worktrees are made by the daemon and recorded by the server

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

Agents changed Brian's normal checkouts, or worked in throwaway worktrees under the task's folder that vanished after each task (`execenv/local_worktree.go`). UJ-2 gives an issue without a worklane a worktree per repository that lasts as long as the issue. UJ-3 makes a worklane's worktree step by step, and UJ-6 switches a worklane's worktree to its next issue's branch after a merge. The server cannot reach a runtime's disk; it reaches the daemon only through heartbeats, claims and pending-work hints.

## Decision

- Only the daemon touches a runtime's disk. Checking a checkout, cloning, making, switching and removing a worktree run on the daemon that holds the repository's checkout, as a request the server enqueues and the daemon answers. The UI waits on the request.
- The server records every worktree a daemon made, with its machine, path and branch. The record is the authority for where a worktree is. A task reuses the recorded worktree; it never recomputes the path.
- A task never writes in a repository's normal checkout. It works in a worktree of that checkout on its own branch, cut from `origin/{default branch}` after a fetch.
- A worktree is removed only when it holds no uncommitted change and no commit missing from its remote. Otherwise it stays, and the reason is said where Brian sees it: on the issue, or on the worklane's activity.
- Removal is asked by the server and done by the daemon at its next heartbeat, so an offline machine catches up when it is back.
- Two tasks never run in one worktree at once: the daemon locks the worktree path.

## Consequences

- Project `local_directory` resources, their in-place and worktree modes, and the per-task snapshot replay go.
- UJ-3's Create worklane and UJ-6's switch after a merge reuse the request round trip and the "nothing would be lost" check.
- A worktree a daemon made but failed to report is not known to the server; the next task on the issue finds it at the computed path and reports it.
- Removing a repository, project or issue deletes its records and leaves the folders on disk.
