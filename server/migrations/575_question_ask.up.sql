-- An agent asks one question or a set of several at once (design.md decisions
-- 1 and 3, ADR-0004). An ask is a group key on question, not a table:
--   ask_id         the id of the ask's first question; ask_id = id for a
--                  one-question ask
--   ask_position   the question's 0-based place in its ask
--   ask_title      the set's title, set only when the ask holds several
--   answer_policy  copied from the ask to each question: under any_one the
--                  first answer answers it, under each every current answerer
--                  must answer; partly_answered is each's state in between
--
-- Every existing question becomes a one-question ask under any_one, which is
-- how it already behaves. No foreign key (AGENTS.md): ask_id names a question
-- of the same table and the service writes a whole ask in one transaction.
-- The (ask_id, ask_position) index is built CONCURRENTLY in 576.
--
-- The runner sends this file as one implicit transaction. ALTER TABLE takes a
-- short ACCESS EXCLUSIVE lock on question, so bound the wait and retry on the
-- next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

ALTER TABLE question
    ADD COLUMN IF NOT EXISTS ask_id        UUID,
    ADD COLUMN IF NOT EXISTS ask_position  INTEGER,
    ADD COLUMN IF NOT EXISTS ask_title     TEXT,
    ADD COLUMN IF NOT EXISTS answer_policy TEXT NOT NULL DEFAULT 'any_one';

UPDATE question
SET ask_id = COALESCE(ask_id, id),
    ask_position = COALESCE(ask_position, 0)
WHERE ask_id IS NULL OR ask_position IS NULL;

ALTER TABLE question
    ALTER COLUMN ask_id SET NOT NULL,
    ALTER COLUMN ask_position SET NOT NULL;

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_answer_policy_check;
ALTER TABLE question ADD CONSTRAINT question_answer_policy_check
    CHECK (answer_policy IN ('any_one', 'each'));

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_status_check;
ALTER TABLE question ADD CONSTRAINT question_status_check
    CHECK (status IN ('open', 'partly_answered', 'answered', 'closed', 'withdrawn'));
