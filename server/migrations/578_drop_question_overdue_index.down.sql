-- Restore migration 570's open-only index before rollback removes migration
-- 577's broader replacement.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_overdue
    ON question (due_date)
    WHERE status = 'open' AND overdue_notified_at IS NULL;
