-- A squad member can be a squad (ADR-0017): squad_member.member_type widens
-- from agent and member to squad. No table, column or data changes; no
-- foreign key (AGENTS.md): the handler loads the sub-squad in its workspace.
--
-- The CHECK is dropped and added back NOT VALID, then validated. Widening a
-- CHECK cannot fail on existing rows. The runner sends this file as one
-- implicit transaction, so bound the wait for the short ACCESS EXCLUSIVE lock
-- and retry on the next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

ALTER TABLE squad_member DROP CONSTRAINT IF EXISTS squad_member_member_type_check;
ALTER TABLE squad_member ADD CONSTRAINT squad_member_member_type_check
    CHECK (member_type IN ('agent', 'member', 'squad')) NOT VALID;
ALTER TABLE squad_member VALIDATE CONSTRAINT squad_member_member_type_check;
