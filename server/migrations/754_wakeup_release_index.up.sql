CREATE INDEX CONCURRENTLY IF NOT EXISTS issue_wakeup_release_idx ON issue_wakeup (release_id, workspace_id) WHERE release_id IS NOT NULL;
