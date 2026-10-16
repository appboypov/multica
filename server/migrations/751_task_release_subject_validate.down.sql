-- PostgreSQL cannot mark a validated constraint NOT VALID again; recreate the
-- state migration 750 left. Both statements are catalog-only.
ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_queue_release_subject_check;
ALTER TABLE agent_task_queue ADD CONSTRAINT agent_task_queue_release_subject_check CHECK (
    release_id IS NULL
    OR (issue_id IS NULL AND chat_session_id IS NULL AND autopilot_run_id IS NULL)
) NOT VALID;
