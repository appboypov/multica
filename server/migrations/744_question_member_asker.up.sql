SET LOCAL lock_timeout = '2s';

ALTER TABLE question
    ALTER COLUMN asker_agent_id DROP NOT NULL,
    ALTER COLUMN asked_in_task_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS asker_user_id UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'question'::regclass AND conname = 'question_one_asker'
    ) THEN
        ALTER TABLE question ADD CONSTRAINT question_one_asker CHECK (
            (asker_agent_id IS NOT NULL AND asked_in_task_id IS NOT NULL AND asker_user_id IS NULL)
            OR
            (asker_agent_id IS NULL AND asked_in_task_id IS NULL AND asker_user_id IS NOT NULL)
        );
    END IF;
END $$;
