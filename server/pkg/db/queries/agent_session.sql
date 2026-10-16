-- Agent sessions are the sessions an agent was started in by hand,
-- registered and followed by the daemon of their machine. Every query filters
-- by workspace_id, the tenant guard; the daemon's writes also by daemon_id, so
-- a daemon only changes the sessions it follows.

-- name: UpsertAgentSession :one
-- The daemon registers a session. A second register of the same process on the
-- same machine replaces the row, as a new session started now. A row of another
-- workspace is never taken over: the conflict then returns no row.
INSERT INTO agent_session (
    workspace_id, agent_id, daemon_id, pid, process_started_at,
    herdr_pane_id, work_dir, repository, branch, status
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('agent_id'), sqlc.arg('daemon_id'),
    sqlc.arg('pid'), sqlc.arg('process_started_at'), sqlc.narg('herdr_pane_id'),
    sqlc.arg('work_dir'), sqlc.narg('repository'), sqlc.narg('branch'),
    sqlc.arg('status')
)
ON CONFLICT (daemon_id, pid) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    process_started_at = EXCLUDED.process_started_at,
    herdr_pane_id = EXCLUDED.herdr_pane_id,
    work_dir = EXCLUDED.work_dir,
    repository = EXCLUDED.repository,
    branch = EXCLUDED.branch,
    status = EXCLUDED.status,
    started_at = now(),
    updated_at = now()
WHERE agent_session.workspace_id = EXCLUDED.workspace_id
RETURNING *;

-- name: UpdateAgentSession :one
-- The daemon saw the session's status, pane, repository or branch change. Only the
-- daemon that follows the session changes it.
UPDATE agent_session SET
    status = sqlc.arg('status'),
    repository = sqlc.narg('repository'),
    branch = sqlc.narg('branch'),
    herdr_pane_id = COALESCE(sqlc.narg('herdr_pane_id'), herdr_pane_id),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
  AND daemon_id = sqlc.arg('daemon_id')
RETURNING *;

-- name: DeleteAgentSession :one
-- The session ended. Only the daemon that follows it ends it; the agent is
-- returned so the caller announces agent:sessions.
DELETE FROM agent_session
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
  AND daemon_id = sqlc.arg('daemon_id')
RETURNING agent_id;

-- name: ListAgentSessionsForDaemon :many
-- The sessions a daemon follows (agent_session_daemon_pid_unique), for its
-- sweep and for dropping what ended while it was down.
SELECT * FROM agent_session
WHERE daemon_id = sqlc.arg('daemon_id')
  AND workspace_id = sqlc.arg('workspace_id')
ORDER BY started_at ASC, id ASC;

-- name: ListAgentSessions :many
-- An agent's active sessions: its dispatched and running tasks as runs, and
-- its sessions started by hand on a daemon that is online. The non-lane run arm
-- comes first because the result's column types follow it.
--
-- A worklane run uses its recorded lane's repository and the lowercase issue
-- key as branch, only when both lane and repository belong to the workspace.
-- Without a lane, use the issue's active worktree at the task's folder, else
-- its only active worktree. Several active worktrees elsewhere leave both empty.
-- agent_task_queue has no workspace_id, so the agent carries the tenant guard.
SELECT
    t.id AS id,
    'run'::text AS kind,
    NULL::integer AS pid,
    NULL::text AS herdr_pane_id,
    rt.id AS runtime_id,
    rt.name AS runtime_name,
    t.id AS task_id,
    i.id AS issue_id,
    COALESCE(ws.issue_prefix || '-' || i.number, '')::text AS issue_identifier,
    COALESCE(t.work_dir, '')::text AS work_dir,
    repo.full_name AS repository,
    iw.branch AS branch,
    (CASE WHEN t.status = 'running' THEN 'working' ELSE 'starting' END)::text AS status,
    COALESCE(t.started_at, t.dispatched_at) AS started_at
FROM agent_task_queue t
JOIN agent a ON a.id = t.agent_id AND a.workspace_id = sqlc.arg('workspace_id')
JOIN workspace ws ON ws.id = a.workspace_id
LEFT JOIN agent_runtime rt ON rt.id = t.runtime_id AND rt.workspace_id = a.workspace_id
LEFT JOIN issue i ON i.id = t.issue_id AND i.workspace_id = a.workspace_id
LEFT JOIN issue_worktree iw ON iw.id = (
    SELECT w.id FROM issue_worktree w
    WHERE w.issue_id = i.id
      AND w.workspace_id = a.workspace_id
      AND w.state = 'active'
      AND (
          w.path = t.work_dir
          OR NOT EXISTS (
              SELECT 1 FROM issue_worktree other
              WHERE other.issue_id = w.issue_id
                AND other.workspace_id = w.workspace_id
                AND other.state = 'active'
                AND other.id <> w.id
          )
      )
    ORDER BY w.created_at ASC, w.id ASC
    LIMIT 1
)
LEFT JOIN repository repo ON repo.id = iw.repository_id AND repo.workspace_id = a.workspace_id
WHERE t.agent_id = sqlc.arg('agent_id')
  AND t.status IN ('dispatched', 'running')
  AND i.worklane_id IS NULL
UNION ALL
SELECT
    t.id,
    'run'::text,
    NULL::integer,
    NULL::text,
    rt.id,
    rt.name,
    t.id,
    i.id,
    (ws.issue_prefix || '-' || i.number)::text,
    COALESCE(t.work_dir, '')::text,
    repo.full_name,
    CASE WHEN repo.id IS NOT NULL THEN lower(ws.issue_prefix || '-' || i.number::text) END,
    (CASE WHEN t.status = 'running' THEN 'working' ELSE 'starting' END)::text,
    COALESCE(t.started_at, t.dispatched_at)
FROM agent_task_queue t
JOIN agent a ON a.id = t.agent_id AND a.workspace_id = sqlc.arg('workspace_id')
JOIN workspace ws ON ws.id = a.workspace_id
LEFT JOIN agent_runtime rt ON rt.id = t.runtime_id AND rt.workspace_id = a.workspace_id
JOIN issue i ON i.id = t.issue_id AND i.workspace_id = a.workspace_id
LEFT JOIN worklane wl ON wl.id = i.worklane_id AND wl.workspace_id = a.workspace_id
LEFT JOIN repository repo ON repo.id = wl.repository_id AND repo.workspace_id = a.workspace_id
WHERE t.agent_id = sqlc.arg('agent_id')
  AND t.status IN ('dispatched', 'running')
  AND i.worklane_id IS NOT NULL
UNION ALL
SELECT
    s.id,
    'terminal'::text,
    s.pid,
    s.herdr_pane_id,
    NULL::uuid,
    NULL::text,
    NULL::uuid,
    NULL::uuid,
    ''::text,
    s.work_dir,
    s.repository,
    s.branch,
    s.status,
    s.started_at
FROM agent_session s
WHERE s.workspace_id = sqlc.arg('workspace_id')
  AND s.agent_id = sqlc.arg('agent_id')
  AND EXISTS (
      SELECT 1 FROM agent_runtime r
      WHERE r.workspace_id = s.workspace_id
        AND r.daemon_id = s.daemon_id
        AND r.status = 'online'
  )
ORDER BY started_at ASC, id ASC;
