-- Repository-wide entries, shown on every worklane of the repository
-- (ADR-0008). Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS activity_log_repository_keyset_idx
    ON activity_log (repository_id, created_at DESC, id DESC)
    WHERE repository_id IS NOT NULL AND worklane_id IS NULL;
