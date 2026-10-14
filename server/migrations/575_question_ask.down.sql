-- Reverse 575: UJ-1's status CHECK has no partly_answered, and moving such a
-- question to open or answered would invent a status its answers never gave
-- it. Refuse while one exists; settle those questions (or roll back the
-- application only and keep these columns, which older builds ignore) first.
--
-- The guard aborts before any DDL runs. Rolling back otherwise turns every
-- set into separate questions and every Each question into an Any one
-- question; no question, answer or event is deleted.
SET LOCAL lock_timeout = '2s';

DO $$
DECLARE
    partly BIGINT;
BEGIN
    SELECT count(*) INTO partly
    FROM question
    WHERE status = 'partly_answered';

    IF partly > 0 THEN
        RAISE EXCEPTION 'cannot roll back 575: % question(s) are partly_answered', partly
            USING HINT = 'Close or finish answering those questions first, or roll back the application only and keep the ask columns.';
    END IF;
END
$$;

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_status_check;
ALTER TABLE question ADD CONSTRAINT question_status_check
    CHECK (status IN ('open', 'answered', 'closed', 'withdrawn'));

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_answer_policy_check;

ALTER TABLE question
    DROP COLUMN IF EXISTS answer_policy,
    DROP COLUMN IF EXISTS ask_title,
    DROP COLUMN IF EXISTS ask_position,
    DROP COLUMN IF EXISTS ask_id;
