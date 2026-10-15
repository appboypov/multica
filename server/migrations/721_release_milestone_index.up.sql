CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_release_milestone_release ON release_milestone (release_id, date, name);
