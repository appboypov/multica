-- The list of an agent's sessions reads its rows.
CREATE INDEX CONCURRENTLY IF NOT EXISTS agent_session_workspace_agent_idx
    ON agent_session (workspace_id, agent_id);
