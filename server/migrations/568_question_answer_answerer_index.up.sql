-- One answer per answerer, edited in place: the arbiter of the answer upsert's
-- ON CONFLICT. Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_question_answer_answerer
    ON question_answer (question_id, answerer_user_id);
