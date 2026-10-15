-- 721 down builds this index concurrently before the key is restored.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '10s';

ALTER TABLE task_supplement
    ADD CONSTRAINT task_supplement_pkey PRIMARY KEY USING INDEX task_supplement_comment_task_uidx,
    DROP COLUMN handled_at,
    DROP COLUMN seen_at,
    DROP COLUMN content,
    DROP COLUMN item_id,
    DROP COLUMN item_kind;
