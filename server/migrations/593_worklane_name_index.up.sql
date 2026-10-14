-- A worklane's name is unique in its repository, ignoring case.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS worklane_repository_name_unique
    ON worklane (repository_id, LOWER(name));
