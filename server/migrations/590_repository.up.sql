-- Repositories hold worklanes (ADR-0006). A repository is a GitHub repository
-- in one project: its GitHub fields are read from GitHub by the server, and its
-- normal checkout is a machine (the daemon id) and an absolute path accepted
-- only after a check on that machine passed. Every repository has exactly one
-- worklane `main`, which works in that checkout and stores no folder or machine
-- of its own; UJ-3's worklanes fill folder and daemon_id.
--
-- issue_worktree records each worktree a daemon made for an issue (ADR-0007):
-- the row is the authority for where the worktree is, so a later key change
-- moves nothing. It is `removing` while the daemon is asked to remove it and
-- `kept` with its reason when that would lose work.
--
-- No foreign keys or cascades (AGENTS.md): the handlers check every reference
-- against the workspace and delete the levels below in one transaction, and
-- workspace teardown deletes these rows. The indexes are built CONCURRENTLY in
-- 591 to 596, one per file, since CREATE INDEX CONCURRENTLY cannot share this
-- transaction.
CREATE TABLE IF NOT EXISTS repository (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID        NOT NULL,
    project_id         UUID        NOT NULL,
    github_repo_id     BIGINT      NOT NULL,
    full_name          TEXT        NOT NULL,
    private            BOOLEAN     NOT NULL,
    default_branch     TEXT        NOT NULL,
    checkout_daemon_id TEXT        NOT NULL,
    checkout_path      TEXT        NOT NULL,
    version_policy     TEXT        NOT NULL CHECK (version_policy IN (
        'pubspec.yaml', 'Cargo.toml', 'pyproject.toml', 'package.json', 'tags', 'none'
    )),
    created_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS worklane (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID        NOT NULL,
    repository_id UUID        NOT NULL,
    name          TEXT        NOT NULL,
    folder        TEXT,
    daemon_id     TEXT,
    is_main       BOOLEAN     NOT NULL DEFAULT false,
    position      INT         NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS issue_worktree (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID        NOT NULL,
    issue_id      UUID        NOT NULL,
    repository_id UUID        NOT NULL,
    daemon_id     TEXT        NOT NULL,
    path          TEXT        NOT NULL,
    branch        TEXT        NOT NULL,
    state         TEXT        NOT NULL DEFAULT 'active'
        CHECK (state IN ('active', 'removing', 'kept')),
    kept_reason   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
