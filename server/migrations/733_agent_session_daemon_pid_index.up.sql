-- A process on a machine is at most one session: the daemon's register upserts
-- on this pair, and its follower reads its sessions by daemon_id.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS agent_session_daemon_pid_unique
    ON agent_session (daemon_id, pid);
