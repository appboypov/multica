-- Sessions an agent was started in by hand, with a persona command in Herdr or
-- in a plain terminal. The daemon of their machine registers and
-- follows each one: it ends it on Herdr's pane events and on its sweep of the
-- process and branch. A run is never stored here; the list of an agent's
-- sessions reads its dispatched and running tasks at read time.
--
-- daemon_id is the daemon that follows the session, as in
-- agent_runtime.daemon_id. pid is omp's process and process_started_at its
-- start time, so a reused PID is never taken for the session. herdr_pane_id is
-- null outside Herdr; repository (`owner/name` of the folder's origin) and
-- branch are null when the folder has none. status is Herdr's agent status.
--
-- No foreign keys or cascades: the handlers check the agent against
-- the workspace, the daemon deletes the sessions that ended, and workspace
-- teardown deletes these rows. The indexes are built CONCURRENTLY in 733 and
-- 734, one per file, since CREATE INDEX CONCURRENTLY cannot share this
-- transaction.
CREATE TABLE IF NOT EXISTS agent_session (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID        NOT NULL,
    agent_id           UUID        NOT NULL,
    daemon_id          TEXT        NOT NULL,
    pid                INTEGER     NOT NULL,
    process_started_at TIMESTAMPTZ NOT NULL,
    herdr_pane_id      TEXT,
    work_dir           TEXT        NOT NULL,
    repository         TEXT,
    branch             TEXT,
    status             TEXT        NOT NULL DEFAULT 'unknown'
        CHECK (status IN ('idle', 'working', 'blocked', 'done', 'unknown')),
    started_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
