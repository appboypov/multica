-- Stand-up, workcycle and check-out are issue types (ADR-0032). An autopilot
-- creates its issues with its own issue type, so a stand-up autopilot's
-- issue opens with the stand-up template.
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_type_known;
ALTER TABLE issue
    ADD CONSTRAINT issue_type_known CHECK (type IS NULL OR type IN ('epic', 'journey', 'story', 'bug', 'enhancement', 'refactor', 'chore', 'spike', 'standup', 'workcycle', 'checkout')) NOT VALID;

ALTER TABLE autopilot
    ADD COLUMN issue_type text,
    ADD CONSTRAINT autopilot_issue_type_known CHECK (issue_type IS NULL OR issue_type IN ('epic', 'journey', 'story', 'bug', 'enhancement', 'refactor', 'chore', 'spike', 'standup', 'workcycle', 'checkout'));

-- The stand-ups, workcycles and check-outs made before these types existed
-- take their type from their title.
UPDATE issue SET type = 'standup' WHERE title LIKE 'Stand-up %';
UPDATE issue SET type = 'workcycle' WHERE title LIKE 'Workcycle %';
UPDATE issue SET type = 'checkout' WHERE title LIKE 'Check-out %';
UPDATE autopilot SET issue_type = 'standup' WHERE issue_title_template LIKE 'Stand-up %';
