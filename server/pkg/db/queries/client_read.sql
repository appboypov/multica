-- name: GetClientReadWorkspace :one
SELECT workspace_id FROM member
WHERE user_id = sqlc.arg('user_id')::uuid AND role = 'client';

-- name: ListClientWaits :many
SELECT i.id, i.title, i.description, i.due_date, o.name AS organisation_name
FROM issue i
JOIN member m ON m.user_id = sqlc.arg('user_id')::uuid
    AND m.workspace_id = sqlc.arg('workspace_id')::uuid AND m.role = 'client'
JOIN project p ON p.id = i.project_id AND p.workspace_id = sqlc.arg('workspace_id')::uuid
JOIN organisation o ON o.id = p.organisation_id
    AND o.workspace_id = sqlc.arg('workspace_id')::uuid AND o.id = ANY(m.organisation_ids)
WHERE i.workspace_id = sqlc.arg('workspace_id')::uuid
  AND i.type = 'chore'
  AND i.assignee_type = 'member'
  AND i.assignee_id = sqlc.arg('user_id')::uuid
  AND NOT (i.status = ANY(sqlc.arg('terminal_status_keys')::text[]))
  AND EXISTS (
      SELECT 1 FROM issue_to_label itl
      JOIN issue_label l ON l.id = itl.label_id
      WHERE itl.issue_id = i.id
        AND l.workspace_id = sqlc.arg('workspace_id')::uuid
        AND l.resource_type = 'issue'
        AND LOWER(l.name) = 'client wait'
  )
ORDER BY i.due_date ASC NULLS LAST, i.created_at ASC, i.id ASC
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountClientWaits :one
SELECT COUNT(*) FROM issue i
JOIN member m ON m.user_id = sqlc.arg('user_id')::uuid
    AND m.workspace_id = sqlc.arg('workspace_id')::uuid AND m.role = 'client'
JOIN project p ON p.id = i.project_id AND p.workspace_id = sqlc.arg('workspace_id')::uuid
JOIN organisation o ON o.id = p.organisation_id
    AND o.workspace_id = sqlc.arg('workspace_id')::uuid AND o.id = ANY(m.organisation_ids)
WHERE i.workspace_id = sqlc.arg('workspace_id')::uuid
  AND i.type = 'chore'
  AND i.assignee_type = 'member'
  AND i.assignee_id = sqlc.arg('user_id')::uuid
  AND NOT (i.status = ANY(sqlc.arg('terminal_status_keys')::text[]))
  AND EXISTS (
      SELECT 1 FROM issue_to_label itl
      JOIN issue_label l ON l.id = itl.label_id
      WHERE itl.issue_id = i.id
        AND l.workspace_id = sqlc.arg('workspace_id')::uuid
        AND l.resource_type = 'issue'
        AND LOWER(l.name) = 'client wait'
  );

-- name: GetClientWait :one
SELECT i.id, i.title, i.description, i.due_date, o.name AS organisation_name
FROM issue i
JOIN member m ON m.user_id = sqlc.arg('user_id')::uuid
    AND m.workspace_id = sqlc.arg('workspace_id')::uuid AND m.role = 'client'
JOIN project p ON p.id = i.project_id AND p.workspace_id = sqlc.arg('workspace_id')::uuid
JOIN organisation o ON o.id = p.organisation_id
    AND o.workspace_id = sqlc.arg('workspace_id')::uuid AND o.id = ANY(m.organisation_ids)
WHERE i.workspace_id = sqlc.arg('workspace_id')::uuid
  AND i.type = 'chore'
  AND i.assignee_type = 'member'
  AND i.assignee_id = sqlc.arg('user_id')::uuid
  AND NOT (i.status = ANY(sqlc.arg('terminal_status_keys')::text[]))
  AND EXISTS (
      SELECT 1 FROM issue_to_label itl
      JOIN issue_label l ON l.id = itl.label_id
      WHERE itl.issue_id = i.id
        AND l.workspace_id = sqlc.arg('workspace_id')::uuid
        AND l.resource_type = 'issue'
        AND LOWER(l.name) = 'client wait'
  )
  AND i.id = sqlc.arg('id')::uuid;

-- name: ListClientPlannedWork :many
SELECT i.id, i.title, COALESCE(s.name, i.status)::text AS status,
       o.name AS organisation_name, p.title AS project_name,
       r.version, (r.shipped_at IS NOT NULL)::boolean AS shipped
FROM issue i
JOIN member m ON m.user_id = sqlc.arg('user_id')::uuid
    AND m.workspace_id = sqlc.arg('workspace_id')::uuid AND m.role = 'client'
JOIN project p ON p.id = i.project_id AND p.workspace_id = sqlc.arg('workspace_id')::uuid
JOIN organisation o ON o.id = p.organisation_id
    AND o.workspace_id = sqlc.arg('workspace_id')::uuid AND o.id = ANY(m.organisation_ids)
JOIN release r ON r.id = i.release_id AND r.workspace_id = sqlc.arg('workspace_id')::uuid
JOIN repository repo ON repo.id = r.repository_id AND repo.project_id = p.id
    AND repo.workspace_id = sqlc.arg('workspace_id')::uuid
LEFT JOIN issue_status s ON s.workspace_id = sqlc.arg('workspace_id')::uuid AND s.key = i.status
WHERE i.workspace_id = sqlc.arg('workspace_id')::uuid
ORDER BY o.name ASC, o.id ASC, p.title ASC, p.id ASC,
         r.repository_id ASC, r.position ASC, r.id ASC, i.position ASC, i.id ASC
LIMIT sqlc.arg('page_limit')::int OFFSET sqlc.arg('page_offset')::int;

-- name: CountClientPlannedWork :one
SELECT COUNT(*) FROM issue i
JOIN member m ON m.user_id = sqlc.arg('user_id')::uuid
    AND m.workspace_id = sqlc.arg('workspace_id')::uuid AND m.role = 'client'
JOIN project p ON p.id = i.project_id AND p.workspace_id = sqlc.arg('workspace_id')::uuid
JOIN organisation o ON o.id = p.organisation_id
    AND o.workspace_id = sqlc.arg('workspace_id')::uuid AND o.id = ANY(m.organisation_ids)
JOIN release r ON r.id = i.release_id AND r.workspace_id = sqlc.arg('workspace_id')::uuid
JOIN repository repo ON repo.id = r.repository_id AND repo.project_id = p.id
    AND repo.workspace_id = sqlc.arg('workspace_id')::uuid
WHERE i.workspace_id = sqlc.arg('workspace_id')::uuid;
