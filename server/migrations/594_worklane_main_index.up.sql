-- Every repository has exactly one worklane main (ADR-0006).
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS worklane_repository_main_unique
    ON worklane (repository_id) WHERE is_main;
