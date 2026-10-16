-- Rolling back removes the native release subjects. Release discussions,
-- waits and runs have no issue to fall back to, so they are deleted before
-- issue_id becomes required again (see the release-owner migration plan:
-- export retained discussions before a downgrade).
DELETE FROM issue_wakeup_receipt WHERE wakeup_id IN (SELECT id FROM issue_wakeup WHERE release_id IS NOT NULL);
DELETE FROM issue_wakeup WHERE release_id IS NOT NULL;
ALTER TABLE issue_wakeup DROP CONSTRAINT IF EXISTS issue_wakeup_system_rule_check;
ALTER TABLE issue_wakeup ADD CONSTRAINT issue_wakeup_system_rule_check CHECK (system_rule IN ('child_done'));
ALTER TABLE issue_wakeup DROP CONSTRAINT IF EXISTS issue_wakeup_subject_check;
ALTER TABLE issue_wakeup ALTER COLUMN issue_id SET NOT NULL;
ALTER TABLE issue_wakeup DROP COLUMN IF EXISTS release_id;

DELETE FROM attachment WHERE comment_id IN (SELECT id FROM comment WHERE release_id IS NOT NULL);
DELETE FROM comment_reaction WHERE comment_id IN (SELECT id FROM comment WHERE release_id IS NOT NULL);
DELETE FROM comment WHERE release_id IS NOT NULL;
ALTER TABLE comment DROP CONSTRAINT IF EXISTS comment_subject_check;
ALTER TABLE comment ALTER COLUMN issue_id SET NOT NULL;
ALTER TABLE comment DROP COLUMN IF EXISTS release_id;

UPDATE agent_task_queue SET status = 'cancelled', completed_at = now(), error = 'Release coordination removed'
WHERE release_id IS NOT NULL AND status IN ('queued', 'dispatched', 'waiting_local_directory', 'deferred');
ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_queue_release_subject_check;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS release_id;

ALTER TABLE release DROP CONSTRAINT IF EXISTS release_owner_pair_check;
ALTER TABLE release DROP COLUMN IF EXISTS owner_id;
ALTER TABLE release DROP COLUMN IF EXISTS owner_type;
