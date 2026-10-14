-- Placement writes run under the repository line lock in the caller's
-- transaction. Every read and write keeps the workspace tenant guard.

-- name: SetIssueTypeOnce :one
UPDATE issue SET type = sqlc.arg('type'), revision = revision + 1, updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id') AND type IS NULL
RETURNING *;

-- name: UpdateIssuePlacement :one
UPDATE issue SET
    worklane_id = sqlc.narg('worklane_id'),
    release_id = sqlc.narg('release_id'),
    project_id = sqlc.narg('project_id'),
    revision = revision + 1,
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: ListWorklaneIssues :many
SELECT i.id, i.workspace_id, i.title, i.description, i.status, i.priority,
       i.assignee_type, i.assignee_id, i.creator_type, i.creator_id,
       i.parent_issue_id, i.position, i.start_date, i.due_date, i.created_at,
       i.updated_at, i.last_activity_at, i.number, i.project_id, i.metadata, i.stage, i.properties,
       i.revision, i.duplicate_of_issue_id, i.type, i.worklane_id, i.release_id, i.done_at
FROM issue i
WHERE i.workspace_id = sqlc.arg('workspace_id') AND i.worklane_id = sqlc.arg('worklane_id')
ORDER BY i.created_at ASC, i.id ASC;

-- name: ListReleaseProgress :many
SELECT worklane_id, release_id, COUNT(*)::bigint AS total,
       COUNT(*) FILTER (WHERE status = ANY(sqlc.arg('terminal_status_keys')::text[]))::bigint AS done
FROM issue
WHERE workspace_id = sqlc.arg('workspace_id') AND worklane_id IS NOT NULL AND release_id IS NOT NULL
GROUP BY worklane_id, release_id
ORDER BY worklane_id, release_id;

-- name: ListIssuesForRelease :many
-- Locks selected issues before the caller checks that they can enter a release.
SELECT i.id, (w.issue_prefix || '-' || i.number)::text AS identifier, i.worklane_id, i.release_id
FROM issue i
JOIN workspace w ON w.id = i.workspace_id
WHERE i.workspace_id = sqlc.arg('workspace_id') AND i.id = ANY(sqlc.arg('issue_ids')::uuid[])
ORDER BY i.id
FOR UPDATE OF i;

-- name: SetReleaseOfIssues :many
UPDATE issue SET release_id = sqlc.arg('release_id'), revision = revision + 1, updated_at = now()
WHERE workspace_id = sqlc.arg('workspace_id') AND id = ANY(sqlc.arg('issue_ids')::uuid[])
RETURNING id;

-- name: ClearReleaseOfIssues :many
UPDATE issue SET release_id = NULL, revision = revision + 1, updated_at = now()
WHERE workspace_id = sqlc.arg('workspace_id') AND release_id = sqlc.arg('release_id')
RETURNING id;

-- name: ListOpenIssuesOfWorklane :many
SELECT id, number, title
FROM issue
WHERE workspace_id = sqlc.arg('workspace_id') AND worklane_id = sqlc.arg('worklane_id')
  AND NOT (status = ANY(sqlc.arg('terminal_status_keys')::text[]))
ORDER BY created_at ASC, id ASC;

-- name: ClearWorklaneOfIssues :many
-- Only closed issues may leave a worklane being removed. Keep their release.
UPDATE issue SET worklane_id = NULL, revision = revision + 1, updated_at = now()
WHERE workspace_id = sqlc.arg('workspace_id') AND worklane_id = sqlc.arg('worklane_id')
  AND status = ANY(sqlc.arg('terminal_status_keys')::text[])
RETURNING id;

-- name: ClearPlacementOfRepositoryIssues :many
-- Runs before the worklanes are deleted.
UPDATE issue SET worklane_id = NULL, release_id = NULL, revision = revision + 1, updated_at = now()
WHERE issue.workspace_id = sqlc.arg('workspace_id')
  AND issue.worklane_id IN (SELECT wl.id FROM worklane wl
                           WHERE wl.workspace_id = sqlc.arg('workspace_id') AND wl.repository_id = sqlc.arg('repository_id'))
RETURNING issue.id;

-- name: ClearPlacementOfProjectIssues :many
-- Runs before the repositories and their worklanes are deleted.
UPDATE issue SET worklane_id = NULL, release_id = NULL, revision = revision + 1, updated_at = now()
WHERE issue.workspace_id = sqlc.arg('workspace_id')
  AND issue.worklane_id IN (SELECT wl.id FROM worklane wl
                     JOIN repository r ON r.id = wl.repository_id AND r.workspace_id = wl.workspace_id
                     WHERE wl.workspace_id = sqlc.arg('workspace_id') AND r.project_id = sqlc.arg('project_id'))
RETURNING issue.id;

-- name: ListReleaseTotals :many
-- Closed issues of a removed worklane keep their release, so totals cannot
-- be derived by summing the worklane progress rows.
SELECT release_id, COUNT(*)::bigint AS total,
       COUNT(*) FILTER (WHERE status = ANY(sqlc.arg('terminal_status_keys')::text[]))::bigint AS done
FROM issue
WHERE workspace_id = sqlc.arg('workspace_id') AND release_id IS NOT NULL
GROUP BY release_id
ORDER BY release_id;
