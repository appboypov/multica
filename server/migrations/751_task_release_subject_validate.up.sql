-- Validate the CHECK migration 750 added NOT VALID, under SHARE UPDATE
-- EXCLUSIVE so readers and writers of agent_task_queue continue. Every
-- existing row has release_id NULL and passes.
ALTER TABLE agent_task_queue VALIDATE CONSTRAINT agent_task_queue_release_subject_check;
