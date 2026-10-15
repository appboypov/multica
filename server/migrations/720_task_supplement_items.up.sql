ALTER TABLE task_supplement
    ADD COLUMN item_kind TEXT NOT NULL DEFAULT 'comment'
        CHECK (item_kind IN ('comment', 'comment_edit', 'answer', 'issue_change')),
    ADD COLUMN item_id UUID,
    ADD COLUMN content TEXT,
    ADD COLUMN seen_at TIMESTAMPTZ,
    ADD COLUMN handled_at TIMESTAMPTZ;

UPDATE task_supplement SET item_id = comment_id;

ALTER TABLE task_supplement ALTER COLUMN item_id SET NOT NULL;
