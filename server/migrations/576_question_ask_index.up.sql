-- Loads an ask's questions in order (ListQuestionsByAsk). Single statement +
-- CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_ask
    ON question (ask_id, ask_position);
