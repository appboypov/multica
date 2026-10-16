CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS issue_wakeup_release_system_rule_idx ON issue_wakeup (release_id, system_rule) WHERE system_rule IS NOT NULL AND release_id IS NOT NULL;
