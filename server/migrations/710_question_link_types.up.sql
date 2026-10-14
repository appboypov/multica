-- A question links to any workspace object (design.md decision 1, ADR-0007):
-- question_link.link_type widens from UJ-1's issue and task to the eight types
-- of the registry in server/internal/question/links.go, and question_event
-- gains the kind linked, one event per link an agent adds after asking.
-- No table, column or data changes; no foreign key (AGENTS.md): the service
-- loads each linked object in its workspace and the deleting transactions drop
-- links to deleted objects.
--
-- Both CHECKs are dropped and added back NOT VALID, then validated. Widening
-- a CHECK cannot fail on existing rows. The runner sends this file as one
-- implicit transaction, so bound the wait for the short ACCESS EXCLUSIVE lock
-- and retry on the next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

ALTER TABLE question_link DROP CONSTRAINT IF EXISTS question_link_link_type_check;
ALTER TABLE question_link ADD CONSTRAINT question_link_link_type_check
    CHECK (link_type IN (
        'issue', 'task', 'comment', 'project', 'agent', 'squad', 'autopilot', 'skill'
    )) NOT VALID;
ALTER TABLE question_link VALIDATE CONSTRAINT question_link_link_type_check;

ALTER TABLE question_event DROP CONSTRAINT IF EXISTS question_event_kind_check;
ALTER TABLE question_event ADD CONSTRAINT question_event_kind_check
    CHECK (kind IN (
        'asked', 'answered', 'answer_edited', 'ask_back', 'reassigned', 'closed', 'withdrawn', 'woken',
        'linked'
    )) NOT VALID;
ALTER TABLE question_event VALIDATE CONSTRAINT question_event_kind_check;
