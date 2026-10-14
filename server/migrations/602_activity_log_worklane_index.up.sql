-- A worklane's activity, newest first in keyset order (ADR-0008).
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS activity_log_worklane_keyset_idx
    ON activity_log (worklane_id, created_at DESC, id DESC)
    WHERE worklane_id IS NOT NULL;
