CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_todo_open
ON todo (workspace_id, member_id, kind, source_key)
WHERE done_at IS NULL;
