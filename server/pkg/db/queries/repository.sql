-- Repositories of a project (ADR-0006). Every query filters by workspace_id,
-- the tenant guard: a repository of another workspace reads as absent.

-- name: ListRepositories :many
SELECT * FROM repository
WHERE workspace_id = $1
ORDER BY LOWER(full_name) ASC;

-- name: ListRepositoriesByProject :many
SELECT * FROM repository
WHERE project_id = $1 AND workspace_id = $2
ORDER BY LOWER(full_name) ASC;

-- name: GetRepositoryInWorkspace :one
SELECT * FROM repository
WHERE id = $1 AND workspace_id = $2;

-- name: CreateRepository :one
-- A repository already in a project fails on
-- repository_workspace_full_name_unique; the handler answers 409 with that
-- project. The GitHub fields come from GitHub and the checkout from a passed
-- check, never from the client. install_command is NULL while not found yet,
-- '' for None.
INSERT INTO repository (
    workspace_id, project_id, github_repo_id, full_name, private,
    default_branch, checkout_daemon_id, checkout_path, version_policy,
    install_command, created_by
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('project_id'), sqlc.arg('github_repo_id'),
    sqlc.arg('full_name'), sqlc.arg('private'), sqlc.arg('default_branch'),
    sqlc.arg('checkout_daemon_id'), sqlc.arg('checkout_path'),
    sqlc.arg('version_policy'), sqlc.narg('install_command'),
    sqlc.narg('created_by')
)
RETURNING *;

-- name: UpdateRepositoryCheckout :one
-- Only a passed check moves the checkout. Worklane main follows, since it
-- reads its worktree through the repository.
UPDATE repository SET
    checkout_daemon_id = sqlc.arg('checkout_daemon_id'),
    checkout_path = sqlc.arg('checkout_path'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: UpdateRepositoryGithub :one
-- The fields GitHub owns, read again on every edit. full_name and
-- github_repo_id never change: the repository is fixed.
UPDATE repository SET
    private = sqlc.arg('private'),
    default_branch = sqlc.arg('default_branch'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: UpdateRepositoryVersionPolicy :one
UPDATE repository SET
    version_policy = sqlc.arg('version_policy'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: UpdateRepositoryInstallCommand :one
-- The command a new worklane runs in its worktree: '' is None.
UPDATE repository SET
    install_command = sqlc.arg('install_command'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: BackfillRepositoryInstallCommand :many
-- A passed check fills the install command of a repository from before this
-- column, when it names that repository and its checkout. Returns the rows it
-- set so the handler publishes repository:updated for each.
UPDATE repository SET
    install_command = sqlc.arg('install_command'),
    updated_at = now()
WHERE workspace_id = sqlc.arg('workspace_id')
  AND LOWER(full_name) = LOWER(sqlc.arg('full_name'))
  AND checkout_daemon_id = sqlc.arg('checkout_daemon_id')
  AND checkout_path = sqlc.arg('checkout_path')
  AND install_command IS NULL
RETURNING *;

-- name: DeleteRepository :exec
-- The handler deletes the repository's worklanes in the same transaction. Its
-- issue worktrees stay recorded, so they are still removed when their issue
-- closes.
DELETE FROM repository WHERE id = $1 AND workspace_id = $2;

-- name: DeleteRepositoriesByProject :exec
-- Project delete runs this after DeleteWorklanesByProject, in one transaction.
DELETE FROM repository WHERE project_id = $1 AND workspace_id = $2;
