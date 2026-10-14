-- Whether agents may ask questions on this issue (DAS-431). NULL follows the
-- workspace default (workspace.settings "agent_questions", absent = true),
-- true is On, false is Off. The value is read once, when a task is claimed.
--
-- A nullable column with no default is a catalog-only change, so this does not
-- rewrite the table. Bound lock acquisition so the ALTER fails fast instead of
-- queueing an ACCESS EXCLUSIVE lock in front of every issue query.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '10s';

ALTER TABLE issue ADD COLUMN IF NOT EXISTS agent_questions boolean;
