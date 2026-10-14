-- A folder holds at most one worklane of the repository; the create request
-- refuses a taken folder with folder_taken (ADR-0009). `main` has no folder.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS worklane_repository_folder_unique
    ON worklane (repository_id, folder) WHERE folder IS NOT NULL;
