-- The hourly overdue sweep: open questions past their due date that have not
-- had their overdue notice. Partial, so stamped and settled questions leave it.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_overdue
    ON question (due_date)
    WHERE status = 'open' AND overdue_notified_at IS NULL;
