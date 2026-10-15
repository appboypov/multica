CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_todo_list
ON todo (workspace_id, member_id, created_at DESC)
WHERE done_at IS NULL;
