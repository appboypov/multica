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

-- name: LockReleaseLineOwners :one
-- Fence workspace/project teardown before taking the repository lock.
SELECT repository.id FROM repository
JOIN project ON project.id = repository.project_id AND project.workspace_id = repository.workspace_id
JOIN workspace ON workspace.id = repository.workspace_id
WHERE repository.id = $1 AND repository.workspace_id = $2
FOR KEY SHARE OF workspace, project;

-- name: MarkReleaseShipped :one
UPDATE release SET shipped_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND shipped_at IS NULL
RETURNING *;

-- name: LockReleaseIssuesForShipping :many
SELECT id FROM issue
WHERE workspace_id = $1 AND release_id = $2
ORDER BY id
FOR UPDATE;

-- name: ListReleaseShippingBlockers :many
-- A manual ship records an external publication. Every issue must be Done
-- and every linked pull request must be merged before that record is made.
SELECT i.id, (w.issue_prefix || '-' || i.number)::text AS identifier
FROM issue i JOIN workspace w ON w.id = i.workspace_id
WHERE i.workspace_id = $1 AND i.release_id = $2
  AND (
    issue_effective_status(i.workspace_id, i.status) <> 'done'
    OR EXISTS (
      SELECT 1 FROM issue_pull_request link
      LEFT JOIN github_pull_request pr ON pr.id = link.pull_request_id AND pr.workspace_id = i.workspace_id
      WHERE link.issue_id = i.id AND (pr.id IS NULL OR pr.state <> 'merged')
    )
    OR EXISTS (
      SELECT 1 FROM issue_vcs_pull_request link
      LEFT JOIN vcs_pull_request pr ON pr.id = link.pull_request_id AND pr.workspace_id = i.workspace_id
      WHERE link.issue_id = i.id AND (pr.id IS NULL OR pr.state <> 'merged')
    )
  )
ORDER BY i.number, i.id;
