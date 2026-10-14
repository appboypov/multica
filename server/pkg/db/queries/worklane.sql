-- Worklanes of a repository (ADR-0006). Every query filters by workspace_id,
-- the tenant guard: a worklane of another workspace reads as absent.

-- name: ListWorklanes :many
-- Each repository's main first, then its other worklanes in their order.
SELECT * FROM worklane
WHERE workspace_id = $1
ORDER BY repository_id, is_main DESC, position ASC, LOWER(name) ASC;

-- name: CreateWorklane :one
-- Main is is_main with folder and daemon_id NULL: it works in the
-- repository's checkout. A second main fails on worklane_repository_main_unique,
-- a taken name on worklane_repository_name_unique.
INSERT INTO worklane (
    workspace_id, repository_id, name, folder, daemon_id, is_main, position
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('repository_id'), sqlc.arg('name'),
    sqlc.narg('folder'), sqlc.narg('daemon_id'), sqlc.arg('is_main'),
    sqlc.arg('position')
)
RETURNING *;

-- name: DeleteWorklanesByRepository :many
-- Returns the deleted rows so the handler can publish worklane:deleted.
DELETE FROM worklane
WHERE repository_id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteWorklanesByProject :exec
-- Worklanes reference their repository, never the project, so project delete
-- reaches them through repository; it runs before DeleteRepositoriesByProject.
DELETE FROM worklane
WHERE worklane.workspace_id = sqlc.arg('workspace_id')
  AND worklane.repository_id IN (
    SELECT repository.id FROM repository
    WHERE repository.project_id = sqlc.arg('project_id')
      AND repository.workspace_id = sqlc.arg('workspace_id')
  );

-- name: GetWorklaneInWorkspace :one
SELECT * FROM worklane
WHERE id = $1 AND workspace_id = $2;

-- name: CreateWorklaneWithPath :one
-- A worklane besides main, saved by the create request's last step once its
-- worktree is made. It goes after the repository's other worklanes: its
-- position is the highest plus one, taken in this statement. A taken name
-- fails on worklane_repository_name_unique, a taken folder on
-- worklane_repository_folder_unique.
INSERT INTO worklane (
    workspace_id, repository_id, name, folder, daemon_id, path, is_main, position
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('repository_id'), sqlc.arg('name'),
    sqlc.arg('folder'), sqlc.arg('daemon_id'), sqlc.arg('path'), false,
    (
        SELECT COALESCE(MAX(w.position), 0) + 1 FROM worklane w
        WHERE w.repository_id = sqlc.arg('repository_id')
          AND w.workspace_id = sqlc.arg('workspace_id')
    )
)
RETURNING *;

-- name: RenameWorklane :one
-- Only the name changes: the folder and the worktree stay where they are.
UPDATE worklane SET
    name = sqlc.arg('name'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: DeleteWorklane :exec
-- Runs once the daemon removed the worklane's worktree, with its activity rows
-- in the same transaction.
DELETE FROM worklane WHERE id = $1 AND workspace_id = $2;

-- name: ListWorklanePathsForDaemon :many
-- The worktree paths of a repository's worklanes on one daemon, so an issue
-- worktree never takes a worklane's folder (ADR-0009).
SELECT id, name, path FROM worklane
WHERE workspace_id = sqlc.arg('workspace_id')
  AND repository_id = sqlc.arg('repository_id')
  AND daemon_id = sqlc.arg('daemon_id')
  AND path IS NOT NULL
ORDER BY position ASC;

-- name: CountOtherWorklanes :one
-- The worklanes of a repository besides main: a repository that has any
-- cannot be deleted.
SELECT COUNT(*)::bigint FROM worklane
WHERE repository_id = $1 AND workspace_id = $2 AND NOT is_main;
