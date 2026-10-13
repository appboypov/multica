-- An organisation's key is unique in its workspace: it names the
-- organisation's folder under ~/Worktrees (ADR-0004). The handler tells this
-- 409 apart from the name's by the index name.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS organisation_workspace_key_unique
    ON organisation (workspace_id, key);
