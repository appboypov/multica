-- The project page and project delete read a project's repositories.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS repository_project_idx
    ON repository (project_id);
