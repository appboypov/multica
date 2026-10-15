-- Reverse 732: restore the agent and member CHECK on squad_member.member_type.
-- The narrower CHECK refuses sub-squad rows, so delete them first; rollback
-- loses every sub-squad membership.
--
-- The restored CHECK is validated in this file: a narrowing CHECK must fail
-- closed rather than leave rows it forbids behind an untrusted constraint.
SET LOCAL lock_timeout = '2s';

DELETE FROM squad_member WHERE member_type = 'squad';

ALTER TABLE squad_member DROP CONSTRAINT IF EXISTS squad_member_member_type_check;
ALTER TABLE squad_member ADD CONSTRAINT squad_member_member_type_check
    CHECK (member_type IN ('agent', 'member')) NOT VALID;
ALTER TABLE squad_member VALIDATE CONSTRAINT squad_member_member_type_check;
