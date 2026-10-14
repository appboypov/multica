-- The heartbeat reads the worktrees a daemon is asked to remove.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS issue_worktree_removing_idx
    ON issue_worktree (daemon_id) WHERE state = 'removing';
