CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_agent_task_queue_release ON agent_task_queue (release_id, created_at DESC) WHERE release_id IS NOT NULL;
