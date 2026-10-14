DROP TRIGGER IF EXISTS trg_issue_done_at ON issue;
DROP FUNCTION IF EXISTS issue_done_at();
ALTER TABLE issue DROP COLUMN IF EXISTS done_at;
ALTER TABLE worklane DROP COLUMN IF EXISTS branch;
DELETE FROM activity_log WHERE action IN ('worklane_switched', 'worklane_kept');
