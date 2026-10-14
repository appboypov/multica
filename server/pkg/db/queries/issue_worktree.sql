-- Issue worktrees (ADR-0007): the worktree a daemon made for an issue in one
-- repository. The row is the authority for where it is; a later key change
-- moves nothing. Every query filters by workspace_id, the tenant guard.

-- name: UpsertIssueWorktree :one
-- The daemon records each worktree it made or reused. A second record for the
-- same issue and repository replaces the first and makes it active again.
INSERT INTO issue_worktree (
    workspace_id, issue_id, repository_id, daemon_id, path, branch, state
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('issue_id'), sqlc.arg('repository_id'),
    sqlc.arg('daemon_id'), sqlc.arg('path'), sqlc.arg('branch'), 'active'
)
ON CONFLICT (issue_id, repository_id) DO UPDATE SET
    daemon_id = EXCLUDED.daemon_id,
    path = EXCLUDED.path,
    branch = EXCLUDED.branch,
    state = 'active',
    kept_reason = NULL,
    updated_at = now()
WHERE issue_worktree.workspace_id = EXCLUDED.workspace_id
RETURNING *;

-- name: ListIssueWorktreesForIssue :many
SELECT * FROM issue_worktree
WHERE issue_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;

-- name: SetIssueWorktreesStateForIssue :many
-- removing when the issue becomes Done or Cancelled, active when it leaves
-- them. Returns the rows that changed, so the caller wakes their daemons; a
-- kept row's reason goes with its state.
UPDATE issue_worktree SET
    state = sqlc.arg('state'),
    kept_reason = NULL,
    updated_at = now()
WHERE issue_id = sqlc.arg('issue_id')
  AND workspace_id = sqlc.arg('workspace_id')
  AND state <> sqlc.arg('state')
RETURNING *;

-- name: ListRemovingIssueWorktreesForDaemon :many
-- The heartbeat's removals for one daemon (issue_worktree_removing_idx).
SELECT
    iw.id,
    iw.path,
    iw.branch,
    (w.issue_prefix || '-' || i.number)::text AS issue_identifier
FROM issue_worktree iw
JOIN issue i ON i.id = iw.issue_id AND i.workspace_id = iw.workspace_id
JOIN workspace w ON w.id = iw.workspace_id
WHERE iw.daemon_id = sqlc.arg('daemon_id')
  AND iw.workspace_id = sqlc.arg('workspace_id')
  AND iw.state = 'removing'
ORDER BY iw.created_at ASC;

-- name: ListIssueWorktreePathsForDaemon :many
-- The issue worktrees of a repository on one daemon, so a new worklane never
-- takes an issue's worktree as its own (ADR-0009).
SELECT
    iw.path,
    (w.issue_prefix || '-' || i.number)::text AS issue_identifier
FROM issue_worktree iw
JOIN issue i ON i.id = iw.issue_id AND i.workspace_id = iw.workspace_id
JOIN workspace w ON w.id = iw.workspace_id
WHERE iw.workspace_id = sqlc.arg('workspace_id')
  AND iw.repository_id = sqlc.arg('repository_id')
  AND iw.daemon_id = sqlc.arg('daemon_id')
ORDER BY iw.created_at ASC;

-- name: DeleteIssueWorktree :one
-- The daemon removed the worktree. Only the daemon that holds it resolves it.
DELETE FROM issue_worktree
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
  AND daemon_id = sqlc.arg('daemon_id')
RETURNING *;

-- name: KeepIssueWorktree :one
-- The daemon kept the worktree because removing it would lose work. A row the
-- issue's reopening already made active again stays active.
UPDATE issue_worktree SET
    state = 'kept',
    kept_reason = sqlc.arg('kept_reason'),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
  AND daemon_id = sqlc.arg('daemon_id')
  AND state = 'removing'
RETURNING *;

-- name: DeleteIssueWorktreesByIssue :exec
-- Issue delete forgets its worktrees; the folders stay on disk.
DELETE FROM issue_worktree WHERE issue_id = $1 AND workspace_id = $2;
