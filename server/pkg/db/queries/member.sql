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
INSERT INTO member (workspace_id, user_id, role, organisation_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateMemberRole :one
UPDATE member SET role = $2, organisation_id = $3
WHERE id = $1
RETURNING *;

-- name: DeleteMember :exec
DELETE FROM member WHERE id = $1;

-- name: ListMembersWithUser :many
SELECT m.id, m.workspace_id, m.user_id, m.role, m.organisation_id, m.created_at,
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
       o.id AS organisation_id, o.name AS organisation_name
FROM member m
JOIN workspace w ON w.id = m.workspace_id
LEFT JOIN organisation o ON o.id = m.organisation_id AND o.workspace_id = m.workspace_id
WHERE m.user_id = $1
ORDER BY m.created_at ASC;

-- name: LockUserForMembership :exec
SELECT id FROM "user"
WHERE id = $1
FOR UPDATE;
