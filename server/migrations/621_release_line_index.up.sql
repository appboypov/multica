-- The line reads by repository and position, the workspace list by
-- workspace; every read filters by workspace.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_release_line
    ON release (workspace_id, repository_id, position);
