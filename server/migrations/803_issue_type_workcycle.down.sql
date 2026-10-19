ALTER TABLE autopilot
    DROP CONSTRAINT IF EXISTS autopilot_issue_type_known,
    DROP COLUMN IF EXISTS issue_type;

UPDATE issue SET type = NULL WHERE type IN ('standup', 'workcycle', 'checkout');
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_type_known;
ALTER TABLE issue
    ADD CONSTRAINT issue_type_known CHECK (type IS NULL OR type IN ('epic', 'journey', 'story', 'bug', 'enhancement', 'refactor', 'chore', 'spike')) NOT VALID;
