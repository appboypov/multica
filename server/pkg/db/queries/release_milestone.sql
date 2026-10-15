-- name: ListReleaseMilestones :many
SELECT * FROM release_milestone
WHERE workspace_id = sqlc.arg('workspace_id') AND release_id = ANY(sqlc.arg('release_ids')::uuid[])
ORDER BY date, name, id;

-- name: GetReleaseMilestoneInWorkspace :one
SELECT * FROM release_milestone WHERE id = $1 AND workspace_id = $2;

-- name: CreateReleaseMilestone :one
INSERT INTO release_milestone (workspace_id, release_id, name, date)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateReleaseMilestone :one
UPDATE release_milestone SET name = $3, date = $4, updated_at = now()
WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: DeleteReleaseMilestone :exec
DELETE FROM release_milestone WHERE id = $1 AND workspace_id = $2;

-- name: DeleteReleaseMilestones :exec
DELETE FROM release_milestone WHERE workspace_id = $1 AND release_id = $2;

-- name: DeleteReleaseMilestonesByRepository :exec
DELETE FROM release_milestone
WHERE release_milestone.workspace_id = $1 AND release_milestone.release_id IN (
    SELECT release.id FROM release WHERE release.workspace_id = $1 AND release.repository_id = $2
);

-- name: DeleteReleaseMilestonesByProject :exec
DELETE FROM release_milestone
WHERE release_milestone.workspace_id = $1 AND release_milestone.release_id IN (
    SELECT release.id FROM release JOIN repository ON repository.id = release.repository_id
    WHERE release.workspace_id = $1 AND repository.workspace_id = $1 AND repository.project_id = $2
);

-- name: DeleteReleaseMilestonesByWorkspace :exec
DELETE FROM release_milestone WHERE workspace_id = $1;

-- name: LockReleaseMilestoneOwners :one
-- Fence workspace/project teardown before taking the repository's line lock.
SELECT release.id FROM release
JOIN repository ON repository.id = release.repository_id AND repository.workspace_id = release.workspace_id
JOIN project ON project.id = repository.project_id AND project.workspace_id = release.workspace_id
JOIN workspace ON workspace.id = release.workspace_id
WHERE release.id = $1 AND release.workspace_id = $2
FOR KEY SHARE OF workspace, project;
