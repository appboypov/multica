-- Organisations hold projects (ADR-0004). An organisation is workspace-scoped:
-- a name unique in its workspace ignoring case, and a key unique in its
-- workspace that names the organisation's folder under ~/Worktrees, so it
-- follows the folder-name rule. A project points to at most one organisation
-- through project.organisation_id.
--
-- No foreign keys or cascades (AGENTS.md): the handler checks every
-- organisation_id against the workspace, and workspace teardown deletes these
-- rows. The unique indexes are built CONCURRENTLY in 573 and 574, one per file,
-- since CREATE INDEX CONCURRENTLY cannot share this transaction.
-- project.organisation_id has no index: nothing queries by it.
--
-- The runner sends this file as one implicit transaction. ALTER TABLE takes a
-- short ACCESS EXCLUSIVE lock on project, so bound the wait and retry on the
-- next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

CREATE TABLE IF NOT EXISTS organisation (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL,
    name         TEXT        NOT NULL,
    key          TEXT        NOT NULL CHECK (key ~ '^[a-z0-9][a-z0-9-]*$'),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE project
    ADD COLUMN IF NOT EXISTS organisation_id UUID;
