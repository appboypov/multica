DROP TABLE IF EXISTS project_check_trust;
DROP TABLE IF EXISTS check_settings;
DELETE FROM todo WHERE kind IN ('check', 'trust_proposal');
ALTER TABLE todo DROP CONSTRAINT IF EXISTS todo_kind_check;
ALTER TABLE todo ADD CONSTRAINT todo_kind_check CHECK (kind IN (
    'question', 'mention', 'assigned', 'agent_blocked', 'run_failed',
    'autopilot_paused', 'quota_reached', 'quick_create_failed', 'review'
));
