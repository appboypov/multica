-- Keep the run as provenance while crediting the member it worked for.
UPDATE issue AS i
SET creator_type = 'member',
    creator_id = task.originator_user_id
FROM agent_task_queue AS task
WHERE i.creator_type = 'agent'
  AND i.origin_type IN ('agent_create', 'quick_create')
  AND i.origin_id = task.id
  AND task.originator_user_id IS NOT NULL;
