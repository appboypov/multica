-- Finds the questions a member currently answers: the viewer's answerer rows
-- behind the For me list and the open-question count (ADR-0006).
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_answerer_user
    ON question_answerer (user_id, question_id);
