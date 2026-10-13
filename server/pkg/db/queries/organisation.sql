-- Organisations hold projects (ADR-0004). Every query filters by workspace_id,
-- the tenant guard: an organisation of another workspace reads as absent.

-- name: ListOrganisations :many
SELECT * FROM organisation
WHERE workspace_id = $1
ORDER BY LOWER(name) ASC;

-- name: GetOrganisationInWorkspace :one
SELECT * FROM organisation
WHERE id = $1 AND workspace_id = $2;

-- name: CreateOrganisation :one
-- A taken name or key fails on organisation_workspace_name_unique or
-- organisation_workspace_key_unique, which the handler tells apart.
INSERT INTO organisation (workspace_id, name, key)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateOrganisationKey :one
-- Only the key changes: an organisation is never renamed.
UPDATE organisation SET
    key = sqlc.arg('key'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;
