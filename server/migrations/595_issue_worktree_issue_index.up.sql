-- An issue has one worktree per repository; every task on the issue reuses it
-- (ADR-0007), and the daemon's record upserts on this pair.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_worktree_issue_repository_unique
    ON issue_worktree (issue_id, repository_id);
