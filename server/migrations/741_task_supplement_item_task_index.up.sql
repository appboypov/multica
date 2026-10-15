CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS task_supplement_item_task_key
    ON task_supplement (item_id, task_id);
