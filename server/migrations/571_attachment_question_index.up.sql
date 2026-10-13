-- Answer attachments of a question, as 165_attachment_task_id_index does for
-- tasks. Partial on question_id IS NOT NULL, so the index holds only question
-- uploads. Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_attachment_question
    ON attachment (question_id)
    WHERE question_id IS NOT NULL;
