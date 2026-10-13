-- An organisation's name is unique in its workspace, ignoring case. The handler
-- tells this 409 apart from the key's by the index name.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS organisation_workspace_name_unique
    ON organisation (workspace_id, LOWER(name));
