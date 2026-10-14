-- Lists a workspace's open questions in cursor order for All open (ADR-0006).
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_question_open_workspace
    ON question (workspace_id, created_at, id)
    WHERE status IN ('open', 'partly_answered');
