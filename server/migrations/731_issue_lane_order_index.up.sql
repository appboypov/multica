CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_lane_order ON issue (workspace_id, worklane_id, lane_position, created_at, id) WHERE worklane_id IS NOT NULL;
