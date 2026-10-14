-- Counts the open issues with their own Agent questions setting per
-- workspace. Partial on agent_questions IS NOT NULL, so the index holds only
-- issues off the workspace default. Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_agent_questions
    ON issue (workspace_id)
    WHERE agent_questions IS NOT NULL;
