DELETE FROM task_supplement WHERE item_kind <> 'comment';

SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '10s';

-- 741 down rebuilds the comment key's index; 740 down attaches it.
ALTER TABLE task_supplement
    DROP CONSTRAINT IF EXISTS task_supplement_pkey,
    ALTER COLUMN comment_id SET NOT NULL;
