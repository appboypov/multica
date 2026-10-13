-- Lists the questions linked to one object: an issue's questions for the issue
-- page and the unseen-answer queries. Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_link_target
    ON question_link (link_type, link_id);
