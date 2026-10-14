-- Lists a viewer's answers newest first with the question id as the cursor
-- tie-breaker (ADR-0006); the index supports a backward scan for that order.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_answer_answerer_time
    ON question_answer (answerer_user_id, updated_at, question_id);
