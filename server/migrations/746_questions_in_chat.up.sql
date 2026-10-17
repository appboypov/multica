-- Questions in chat (design.md decisions 1 and 5): a question asked in a chat
-- links to the chat as question_link.link_type 'chat', and a current answerer
-- can cancel a question, an end status with its own event: question.status
-- and question_event.kind gain 'cancelled'. The lists start from those of
-- migrations 710 (link types, event kinds) and 575 (statuses).
-- No table, column or data changes; no foreign key (AGENTS.md): the service
-- loads each chat in its workspace and deleting a chat drops its links.
--
-- The CHECKs are dropped and added back NOT VALID, then validated. Widening
-- a CHECK cannot fail on existing rows. The runner sends this file as one
-- implicit transaction, so bound the wait for the short ACCESS EXCLUSIVE lock
-- and retry on the next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

ALTER TABLE question_link DROP CONSTRAINT IF EXISTS question_link_link_type_check;
ALTER TABLE question_link ADD CONSTRAINT question_link_link_type_check
    CHECK (link_type IN (
        'issue', 'task', 'comment', 'project', 'agent', 'squad', 'autopilot', 'skill',
        'chat'
    )) NOT VALID;
ALTER TABLE question_link VALIDATE CONSTRAINT question_link_link_type_check;

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_status_check;
ALTER TABLE question ADD CONSTRAINT question_status_check
    CHECK (status IN (
        'open', 'partly_answered', 'answered', 'closed', 'withdrawn',
        'cancelled'
    )) NOT VALID;
ALTER TABLE question VALIDATE CONSTRAINT question_status_check;

ALTER TABLE question_event DROP CONSTRAINT IF EXISTS question_event_kind_check;
ALTER TABLE question_event ADD CONSTRAINT question_event_kind_check
    CHECK (kind IN (
        'asked', 'answered', 'answer_edited', 'ask_back', 'reassigned', 'closed', 'withdrawn', 'woken',
        'linked', 'cancelled'
    )) NOT VALID;
ALTER TABLE question_event VALIDATE CONSTRAINT question_event_kind_check;
