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
