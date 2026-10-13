-- A question's history and thread, oldest first. Single statement +
-- CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_event_question
    ON question_event (question_id, created_at);
