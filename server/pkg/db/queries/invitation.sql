-- name: CreateInvitation :one
INSERT INTO workspace_invitation (id, workspace_id, inviter_id, invitee_email, invitee_user_id, role, expires_at, organisation_ids)
VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE(sqlc.narg(organisation_ids)::uuid[], '{}'::uuid[]))
RETURNING *;

-- name: GetInvitationForResponse :one
SELECT wi.id, wi.workspace_id, wi.inviter_id, wi.invitee_email, wi.invitee_user_id,
       wi.role, wi.status, wi.created_at, wi.updated_at, wi.expires_at,
       ARRAY(SELECT o.id FROM organisation o WHERE o.workspace_id = wi.workspace_id AND o.id = ANY(wi.organisation_ids) ORDER BY o.id)::uuid[] AS organisation_ids
FROM workspace_invitation wi
WHERE wi.id = $1;

-- name: GetInvitation :one
SELECT * FROM workspace_invitation
WHERE id = $1;

-- name: ListPendingInvitationsByWorkspace :many
SELECT wi.id, wi.workspace_id, wi.inviter_id, wi.invitee_email, wi.invitee_user_id,
       wi.role, wi.status, wi.created_at, wi.updated_at, wi.expires_at,
       ARRAY(SELECT o.id FROM organisation o WHERE o.workspace_id = wi.workspace_id AND o.id = ANY(wi.organisation_ids) ORDER BY o.id)::uuid[] AS organisation_ids,
       u.name AS inviter_name,
       u.email AS inviter_email
FROM workspace_invitation wi
JOIN "user" u ON u.id = wi.inviter_id
WHERE wi.workspace_id = $1 AND wi.status = 'pending' AND wi.expires_at > now()
ORDER BY wi.created_at DESC;

-- name: ListPendingInvitationsForUser :many
SELECT wi.id, wi.workspace_id, wi.inviter_id, wi.invitee_email, wi.invitee_user_id,
       wi.role, wi.status, wi.created_at, wi.updated_at, wi.expires_at,
       ARRAY(SELECT o.id FROM organisation o WHERE o.workspace_id = wi.workspace_id AND o.id = ANY(wi.organisation_ids) ORDER BY o.id)::uuid[] AS organisation_ids,
       w.name AS workspace_name,
       u.name AS inviter_name,
       u.email AS inviter_email
FROM workspace_invitation wi
JOIN workspace w ON w.id = wi.workspace_id
JOIN "user" u ON u.id = wi.inviter_id
WHERE wi.status = 'pending'
  AND (wi.invitee_user_id = $1 OR wi.invitee_email = $2)
  AND wi.expires_at > now()
ORDER BY wi.created_at DESC;

-- name: AcceptInvitation :one
UPDATE workspace_invitation
SET status = 'accepted', updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: DeclineInvitation :one
UPDATE workspace_invitation
SET status = 'declined', updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: RevokeInvitation :execrows
DELETE FROM workspace_invitation
WHERE id = $1 AND status = 'pending';

-- name: GetPendingInvitationByEmail :one
SELECT * FROM workspace_invitation
WHERE workspace_id = $1 AND invitee_email = $2 AND status = 'pending' AND expires_at > now();

-- name: HasPendingInvitationForEmail :one
SELECT EXISTS (
    SELECT 1
    FROM workspace_invitation
    WHERE invitee_email = $1
      AND status = 'pending'
      AND expires_at > now()
);

-- name: ExpireStalePendingInvitations :many
-- Mark any past-due pending invitations for (workspace_id, invitee_email) as expired,
-- so the next CreateInvitation does not collide with the partial unique index
-- idx_invitation_unique_pending (which is WHERE status = 'pending' and cannot
-- itself reference now() in its predicate).
UPDATE workspace_invitation
SET status = 'expired', updated_at = now()
WHERE workspace_id = $1
  AND invitee_email = $2
  AND status = 'pending'
  AND expires_at <= now()
RETURNING *;
