DELETE FROM activity_log
WHERE action IN ('planned_in_release', 'type_set', 'added_to_release', 'removed_from_release', 'worklane_changed', 'release_gone');
ALTER TABLE issue
    DROP CONSTRAINT IF EXISTS issue_type_known,
    DROP COLUMN IF EXISTS release_id,
    DROP COLUMN IF EXISTS worklane_id,
    DROP COLUMN IF EXISTS type;
