-- An issue gets a type and a place on a worklane and in a release (ADR-0013).
--
-- type NULL is "No type"; existing issues keep NULL and their description.
-- release_id points at the release row, never at a version, so an issue
-- follows its release through every move and renumbering without a write.
-- No foreign keys (AGENTS.md): the handlers clear these columns when a
-- worklane, release, repository or project goes.
--
-- Nullable columns without a default are a catalog-only change. The CHECK is
-- added NOT VALID, as migration 483 does: a validated CHECK would scan every
-- issue under the ALTER's ACCESS EXCLUSIVE lock, while every row on disk is
-- NULL. NOT VALID enforces it on every write from here on.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '10s';

ALTER TABLE issue
    ADD COLUMN type text,
    ADD COLUMN worklane_id uuid,
    ADD COLUMN release_id uuid,
    ADD CONSTRAINT issue_type_known CHECK (type IS NULL OR type IN ('epic', 'journey', 'story', 'bug', 'enhancement', 'refactor', 'chore', 'spike')) NOT VALID;
