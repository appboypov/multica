-- name: ListWorklaneOrder :many
-- The repository line lock serializes placement, release changes and moves.
SELECT id, release_id, lane_position FROM issue
WHERE workspace_id = sqlc.arg('workspace_id') AND worklane_id = sqlc.arg('worklane_id')
ORDER BY lane_position ASC NULLS LAST, created_at ASC, id ASC;

-- name: SetIssueLanePosition :one
UPDATE issue SET lane_position = sqlc.arg('lane_position'), revision = revision + 1, updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id') AND worklane_id = sqlc.arg('worklane_id')
RETURNING *;
