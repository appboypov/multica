-- The release line of a repository (ADR-0010). The server keeps it: each
-- release has its version, set by hand (bump NULL) or computed from its bump
-- after the release before it. position runs 0 to n-1 along the line; the
-- repository row's lock, not a unique index, keeps positions unique, since a
-- move swaps two positions inside one transaction. A release is shipped once
-- shipped_at is set; the first unshipped release is next.
--
-- repository.last_tag: NULL means not known yet, '' means no tag, anything
-- else is a version like 0.4.2, stored without the `v`.
--
-- No foreign keys or cascades (AGENTS.md): the handlers check every reference
-- against the workspace and delete releases with their repository, project or
-- workspace. The index is built CONCURRENTLY in 621.
ALTER TABLE repository ADD COLUMN IF NOT EXISTS last_tag TEXT;

CREATE TABLE IF NOT EXISTS release (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID        NOT NULL,
    repository_id UUID        NOT NULL,
    position      INTEGER     NOT NULL,
    version       TEXT        NOT NULL CHECK (version ~ '^[0-9]+\.[0-9]+\.[0-9]+$'),
    bump          TEXT        CHECK (bump IN ('patch', 'minor', 'major')),
    title         TEXT        NOT NULL CHECK (btrim(title) <> ''),
    description   TEXT        NOT NULL DEFAULT '',
    shipped_at    TIMESTAMPTZ,
    created_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
