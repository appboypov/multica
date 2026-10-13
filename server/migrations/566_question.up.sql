-- Questions agents ask members during an issue run (ADR-0002). A question is
-- its own aggregate of five tables:
--   question           what the asker gave and the status
--   question_link      where it belongs; position 0 is where it was asked
--   question_answerer  the current member answerers
--   question_answer    one answer per answerer, edited in place
--   question_event     its history
-- Answer types, their settings and their values are validated by
-- server/internal/question; the CHECKs below only bound the closed
-- vocabularies. Overdue is derived from due_date, never a status.
--
-- No foreign keys or cascades (AGENTS.md): the application validates every id
-- and workspace teardown deletes these rows. IDs come from the application
-- (dbid.NewV7). The lookup indexes are built CONCURRENTLY in 567-571, one per
-- file, since CREATE INDEX CONCURRENTLY cannot share this transaction.
--
-- question_answer and question_event take clock_timestamp(): events written in
-- one transaction keep their order, and an answer's updated_at is as close to
-- its commit as possible, since "unseen" compares it with a run's dispatched_at.
--
-- The runner sends this file as one implicit transaction. ALTER TABLE takes a
-- short ACCESS EXCLUSIVE lock on attachment, so bound the wait and retry on the
-- next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

CREATE TABLE IF NOT EXISTS question (
    id                  UUID        PRIMARY KEY,
    workspace_id        UUID        NOT NULL,
    asker_agent_id      UUID        NOT NULL,
    asked_in_task_id    UUID        NOT NULL,
    prompt              TEXT        NOT NULL,
    context_line        TEXT,
    answer_type         TEXT        NOT NULL CHECK (answer_type IN (
        'text', 'yes_no', 'single_choice', 'multiple_choice', 'confirm_facts', 'ranking',
        'scale', 'number', 'date', 'person', 'file', 'approval'
    )),
    settings            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    suggested_answer    JSONB,
    status              TEXT        NOT NULL DEFAULT 'open'
                                    CHECK (status IN ('open', 'answered', 'closed', 'withdrawn')),
    due_date            DATE,
    blocks_work         BOOLEAN     NOT NULL DEFAULT false,
    overdue_notified_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS question_link (
    question_id  UUID    NOT NULL,
    workspace_id UUID    NOT NULL,
    link_type    TEXT    NOT NULL CHECK (link_type IN ('issue', 'task')),
    link_id      UUID    NOT NULL,
    position     INTEGER NOT NULL,
    PRIMARY KEY (question_id, link_type, link_id)
);

CREATE TABLE IF NOT EXISTS question_answerer (
    question_id  UUID        NOT NULL,
    workspace_id UUID        NOT NULL,
    user_id      UUID        NOT NULL,
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (question_id, user_id)
);

CREATE TABLE IF NOT EXISTS question_answer (
    id               UUID        PRIMARY KEY,
    question_id      UUID        NOT NULL,
    workspace_id     UUID        NOT NULL,
    answerer_user_id UUID        NOT NULL,
    value            JSONB       NOT NULL,
    note             TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS question_event (
    id           UUID        PRIMARY KEY,
    question_id  UUID        NOT NULL,
    workspace_id UUID        NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN (
        'asked', 'answered', 'answer_edited', 'ask_back', 'reassigned', 'closed', 'withdrawn', 'woken'
    )),
    actor_type   TEXT        NOT NULL,
    actor_id     UUID,
    details      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- Answers of the File and Text types carry attachments uploaded against the
-- question. No foreign key, as attachment.task_id (164).
ALTER TABLE attachment
  ADD COLUMN IF NOT EXISTS question_id UUID;
