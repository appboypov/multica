# ADR-0009: The server keeps worklane and issue worktree folders apart

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

ADR-0006 puts a worklane's worktree at `~/Worktrees/{organisation key}/{repo}/{folder}` and an issue's worktree, for an issue without a worklane, at `~/Worktrees/{organisation key}/{repo}/{issue key}`. Both live in the same folder. A worklane folder is any lowercase name, so Brian can name one `mul-12`, the folder of issue MUL-12. A worktree that is used as it is when it is already at the path (UJ-2 for issues, UJ-3 for worklanes) would then be shared by a worklane and an issue. ADR-0007 makes the server's record the authority for where a worktree is.

## Decision

- The server, which records both kinds of worktree, tells the daemon which paths are taken. A worklane create request carries the repository's issue worktree paths on that machine, and the daemon refuses a worklane path that is one of them.
- An issue's claim carries the repository's worklane paths on that machine. When an issue's default path is one of them, the daemon makes the issue's worktree at `{issue key}-issue` instead, and the server records that path, so later tasks reuse it.
- Worklane folders are not restricted beyond being one lowercase folder name, unique in the repository.

## Consequences

- No worktree is ever both a worklane's and an issue's.
- Brian keeps free folder names; an issue that meets a worklane's folder gets a worktree that is still named after it.
- A worktree made outside Multica at a worklane's path is still used as it is when it is a checkout of the repository; the server cannot know about it.
