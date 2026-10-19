-- Skill folders: folders on one machine whose skills are in the library.

-- name: ListSkillFolders :many
SELECT * FROM skill_folder
WHERE workspace_id = $1
ORDER BY created_at, path;

-- name: ListSkillFoldersByDaemon :many
SELECT * FROM skill_folder
WHERE workspace_id = $1 AND daemon_id = $2
ORDER BY created_at, path;

-- name: GetSkillFolderInWorkspace :one
SELECT * FROM skill_folder
WHERE id = $1 AND workspace_id = $2;

-- name: CreateSkillFolder :one
INSERT INTO skill_folder (workspace_id, daemon_id, path, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteSkillFolder :exec
DELETE FROM skill_folder WHERE id = $1 AND workspace_id = $2;

-- name: ListSkillsByFolder :many
-- A folder's skills are the library rows whose origin names the folder.
SELECT * FROM skill
WHERE workspace_id = $1 AND config->'origin'->>'folder_id' = sqlc.arg('folder_id')::text;
