SET LOCAL lock_timeout = '2s';

DELETE FROM attachment
WHERE question_id IN (SELECT id FROM question WHERE asker_user_id IS NOT NULL);
DELETE FROM question_link
WHERE question_id IN (SELECT id FROM question WHERE asker_user_id IS NOT NULL);
DELETE FROM question_answerer
WHERE question_id IN (SELECT id FROM question WHERE asker_user_id IS NOT NULL);
DELETE FROM question_answer
WHERE question_id IN (SELECT id FROM question WHERE asker_user_id IS NOT NULL);
DELETE FROM question_event
WHERE question_id IN (SELECT id FROM question WHERE asker_user_id IS NOT NULL);
DELETE FROM question WHERE asker_user_id IS NOT NULL;

ALTER TABLE question
    DROP CONSTRAINT IF EXISTS question_one_asker,
    DROP COLUMN IF EXISTS asker_user_id,
    ALTER COLUMN asker_agent_id SET NOT NULL,
    ALTER COLUMN asked_in_task_id SET NOT NULL;
