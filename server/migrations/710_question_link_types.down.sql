-- Reverse 710: restore UJ-1's CHECKs on question_link.link_type and
-- question_event.kind. The narrower CHECKs refuse the rows written under the
-- wider ones, so delete them first: links of the six new types and linked
-- events. Rollback loses those links; their questions keep their issue and
-- task links, which UJ-1 always writes.
--
-- The restored CHECKs are validated in this file: a narrowing CHECK must fail
-- closed rather than leave rows it forbids behind an untrusted constraint.
SET LOCAL lock_timeout = '2s';

DELETE FROM question_link
WHERE link_type IN ('comment', 'project', 'agent', 'squad', 'autopilot', 'skill');

DELETE FROM question_event
WHERE kind = 'linked';

ALTER TABLE question_link DROP CONSTRAINT IF EXISTS question_link_link_type_check;
ALTER TABLE question_link ADD CONSTRAINT question_link_link_type_check
    CHECK (link_type IN ('issue', 'task')) NOT VALID;
ALTER TABLE question_link VALIDATE CONSTRAINT question_link_link_type_check;

ALTER TABLE question_event DROP CONSTRAINT IF EXISTS question_event_kind_check;
ALTER TABLE question_event ADD CONSTRAINT question_event_kind_check
    CHECK (kind IN (
        'asked', 'answered', 'answer_edited', 'ask_back', 'reassigned', 'closed', 'withdrawn', 'woken'
    )) NOT VALID;
ALTER TABLE question_event VALIDATE CONSTRAINT question_event_kind_check;
