-- A release's issues, for its column, Add issue to release and Remove
-- release; every read also filters by workspace.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_release
    ON issue (release_id) WHERE release_id IS NOT NULL;
