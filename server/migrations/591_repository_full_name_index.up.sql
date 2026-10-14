-- A repository is in at most one project of the workspace: its full_name is
-- unique per workspace, ignoring case. The handler names the project that holds
-- it in its 409.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS repository_workspace_full_name_unique
    ON repository (workspace_id, LOWER(full_name));
