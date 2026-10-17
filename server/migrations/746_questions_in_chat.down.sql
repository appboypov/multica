-- Reverse 746: restore migration 710's CHECKs on question_link.link_type and
-- question_event.kind and migration 575's on question.status. The narrower
-- CHECKs refuse the rows written under the wider ones, so clear them first:
-- cancelled questions become withdrawn, the nearest end status before 746,
-- and cancelled events and chat links are deleted. Rollback loses who
-- cancelled, the cancel note and the chat a question was asked in; those
-- questions keep their answers and the run link every ask writes.
--
-- The restored CHECKs are validated in this file: a narrowing CHECK must fail
-- closed rather than leave rows it forbids behind an untrusted constraint.
SET LOCAL lock_timeout = '2s';

UPDATE question
SET status = 'withdrawn'
WHERE status = 'cancelled';

DELETE FROM question_event
WHERE kind = 'cancelled';

DELETE FROM question_link
WHERE link_type = 'chat';

ALTER TABLE question_link DROP CONSTRAINT IF EXISTS question_link_link_type_check;
ALTER TABLE question_link ADD CONSTRAINT question_link_link_type_check
    CHECK (link_type IN (
        'issue', 'task', 'comment', 'project', 'agent', 'squad', 'autopilot', 'skill'
    )) NOT VALID;
ALTER TABLE question_link VALIDATE CONSTRAINT question_link_link_type_check;

ALTER TABLE question DROP CONSTRAINT IF EXISTS question_status_check;
ALTER TABLE question ADD CONSTRAINT question_status_check
    CHECK (status IN ('open', 'partly_answered', 'answered', 'closed', 'withdrawn')) NOT VALID;
ALTER TABLE question VALIDATE CONSTRAINT question_status_check;

ALTER TABLE question_event DROP CONSTRAINT IF EXISTS question_event_kind_check;
ALTER TABLE question_event ADD CONSTRAINT question_event_kind_check
    CHECK (kind IN (
        'asked', 'answered', 'answer_edited', 'ask_back', 'reassigned', 'closed', 'withdrawn', 'woken',
        'linked'
    )) NOT VALID;
ALTER TABLE question_event VALIDATE CONSTRAINT question_event_kind_check;
