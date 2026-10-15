SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '10s';

ALTER TABLE task_supplement
    DROP CONSTRAINT IF EXISTS task_supplement_pkey,
    ADD CONSTRAINT task_supplement_pkey PRIMARY KEY USING INDEX task_supplement_item_task_key,
    ALTER COLUMN comment_id DROP NOT NULL;
