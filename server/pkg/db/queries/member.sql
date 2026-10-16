-- name: ListMembersForResponse :many
SELECT m.id, m.workspace_id, m.user_id, m.role, m.created_at,
       ARRAY(SELECT o.id FROM organisation o WHERE o.workspace_id = m.workspace_id AND o.id = ANY(m.organisation_ids) ORDER BY o.id)::uuid[] AS organisation_ids
FROM member m
WHERE m.workspace_id = $1
ORDER BY created_at ASC;

-- name: ListMembers :many
SELECT * FROM member
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: ListWorkspaceManagerUserIDs :many
SELECT user_id FROM member
WHERE workspace_id = $1 AND role IN ('owner', 'admin')
ORDER BY created_at ASC;

-- name: GetMember :one
SELECT * FROM member
WHERE id = $1;

-- name: GetMemberByUserAndWorkspace :one
SELECT * FROM member
WHERE user_id = $1 AND workspace_id = $2;

-- name: CreateMember :one
INSERT INTO member (workspace_id, user_id, role, organisation_ids)
VALUES ($1, $2, $3, COALESCE(sqlc.narg(organisation_ids)::uuid[], '{}'::uuid[]))
RETURNING *;

-- name: UpdateMemberRole :one
UPDATE member SET role = $2, organisation_ids = COALESCE(sqlc.narg(organisation_ids)::uuid[], '{}'::uuid[])
WHERE id = $1
RETURNING *;

-- name: DeleteMember :exec
DELETE FROM member WHERE id = $1;

-- name: ListMembersWithUser :many
SELECT m.id, m.workspace_id, m.user_id, m.role, m.created_at,
       ARRAY(SELECT o.id FROM organisation o WHERE o.workspace_id = m.workspace_id AND o.id = ANY(m.organisation_ids) ORDER BY o.id)::uuid[] AS organisation_ids,
       u.name as user_name, u.email as user_email, u.avatar_url as user_avatar_url
FROM member m
JOIN "user" u ON u.id = m.user_id
WHERE m.workspace_id = $1
ORDER BY m.created_at ASC;

-- name: IsClientAccount :one
SELECT EXISTS (
    SELECT 1 FROM member
    WHERE user_id = $1 AND role = 'client'
);

-- name: CountMembershipsByUser :one
SELECT COUNT(*) FROM member
WHERE user_id = $1;

-- name: ListMyMemberships :many
SELECT m.workspace_id, w.name AS workspace_name, w.slug AS workspace_slug, m.role,
       ARRAY(SELECT o.id FROM organisation o WHERE m.role = 'client' AND o.workspace_id = m.workspace_id AND o.id = ANY(m.organisation_ids) ORDER BY o.id)::uuid[] AS organisation_ids,
       ARRAY(SELECT o.name FROM organisation o WHERE m.role = 'client' AND o.workspace_id = m.workspace_id AND o.id = ANY(m.organisation_ids) ORDER BY o.id)::text[] AS organisation_names
FROM member m
JOIN workspace w ON w.id = m.workspace_id
WHERE m.user_id = $1
ORDER BY m.created_at ASC;

-- name: GetMemberForResponse :one
SELECT m.id, m.workspace_id, m.user_id, m.role, m.created_at,
       ARRAY(SELECT o.id FROM organisation o WHERE o.workspace_id = m.workspace_id AND o.id = ANY(m.organisation_ids) ORDER BY o.id)::uuid[] AS organisation_ids
FROM member m
WHERE m.id = $1;

-- name: LockUserForMembership :exec
SELECT id FROM "user"
WHERE id = $1
FOR UPDATE;
