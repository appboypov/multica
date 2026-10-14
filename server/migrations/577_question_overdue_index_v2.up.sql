-- The hourly overdue sweep reads open and partly answered questions past their
-- due date that have not had their overdue notice. Migration 570's index covers
-- only open questions; migration 578 drops it once this one exists.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_overdue_v2
    ON question (due_date)
    WHERE status IN ('open', 'partly_answered') AND overdue_notified_at IS NULL;
