-- A worklane's issues, for its Timeline and its remove check; every read
-- also filters by workspace.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_worklane
    ON issue (worklane_id) WHERE worklane_id IS NOT NULL;
