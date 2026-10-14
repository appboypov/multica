-- The release line of a repository (ADR-0010). Every query filters by
-- workspace_id, the tenant guard: a release of another workspace reads as
-- absent. Writes run under LockRepositoryForLine, which keeps positions unique.

-- name: ListReleasesInWorkspace :many
SELECT * FROM release
WHERE workspace_id = $1
ORDER BY repository_id, position ASC;

-- name: ListReleaseLine :many
SELECT * FROM release
WHERE workspace_id = $1 AND repository_id = $2
ORDER BY position ASC;

-- name: GetReleaseInWorkspace :one
SELECT * FROM release
WHERE id = $1 AND workspace_id = $2;

-- name: CreateRelease :one
-- bump NULL means the version was set by hand.
INSERT INTO release (
    workspace_id, repository_id, position, version, bump, title, description,
    created_by
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('repository_id'), sqlc.arg('position'),
    sqlc.arg('version'), sqlc.narg('bump'), sqlc.arg('title'),
    sqlc.arg('description'), sqlc.narg('created_by')
)
RETURNING *;

-- name: UpdateReleaseFields :one
-- A new bump drops the version set by hand; the handler passes the version
-- releaseline computed.
UPDATE release SET
    title = sqlc.arg('title'),
    description = sqlc.arg('description'),
    version = sqlc.arg('version'),
    bump = sqlc.narg('bump'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: UpdateReleasePositionAndVersion :exec
-- Runs for each release of the line whose position or version changed.
UPDATE release SET
    position = sqlc.arg('position'),
    version = sqlc.arg('version'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id');

-- name: DeleteRelease :exec
DELETE FROM release WHERE id = $1 AND workspace_id = $2;

-- name: DeleteReleasesByRepository :exec
-- Repository delete runs this in its transaction.
DELETE FROM release WHERE repository_id = $1 AND workspace_id = $2;

-- name: DeleteReleasesByProject :exec
-- Releases reference their repository, never the project, so project delete
-- reaches them through repository; it runs before DeleteRepositoriesByProject.
DELETE FROM release
WHERE release.workspace_id = sqlc.arg('workspace_id')
  AND release.repository_id IN (
    SELECT repository.id FROM repository
    WHERE repository.project_id = sqlc.arg('project_id')
      AND repository.workspace_id = sqlc.arg('workspace_id')
  );

-- name: ListUnshippedReleaseHeads :many
-- The unshipped releases of a repository, named in the 409
-- has_unshipped_releases refusal of a repository that stops releasing.
SELECT version, title FROM release
WHERE workspace_id = $1 AND repository_id = $2 AND shipped_at IS NULL
ORDER BY position ASC;
