CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS routine_decision_automatic_number
ON routine_decision (workspace_id, automatic_number)
WHERE automatic_number IS NOT NULL;
