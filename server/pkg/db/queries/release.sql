-- The release line of a repository (ADR-0010). Every query filters by
-- workspace_id, the tenant guard: a release of another workspace reads as
-- absent. Writes run under LockRepositoryForLine, which keeps positions unique.

-- name: ListReleasesInWorkspace :many
SELECT * FROM release
WHERE workspace_id = $1
ORDER BY repository_id, position ASC;

-- name: ListReleaseLine :many
SELECT * FROM release
WHERE workspace_id = $1 AND repository_id = $2
ORDER BY position ASC;

-- name: GetReleaseInWorkspace :one
SELECT * FROM release
WHERE id = $1 AND workspace_id = $2;

-- name: CreateRelease :one
-- bump NULL means the version was set by hand.
INSERT INTO release (
    workspace_id, repository_id, position, version, bump, title, description,
    created_by
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('repository_id'), sqlc.arg('position'),
    sqlc.arg('version'), sqlc.narg('bump'), sqlc.arg('title'),
    sqlc.arg('description'), sqlc.narg('created_by')
)
RETURNING *;

-- name: UpdateReleaseFields :one
-- A new bump drops the version set by hand; the handler passes the version
-- releaseline computed.
UPDATE release SET
    title = sqlc.arg('title'),
    description = sqlc.arg('description'),
    version = sqlc.arg('version'),
    bump = sqlc.narg('bump'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: UpdateReleasePositionAndVersion :exec
-- Runs for each release of the line whose position or version changed.
UPDATE release SET
    position = sqlc.arg('position'),
    version = sqlc.arg('version'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id');

-- name: DeleteRelease :exec
DELETE FROM release WHERE id = $1 AND workspace_id = $2;

-- name: DeleteReleasesByRepository :exec
-- Repository delete runs this in its transaction.
DELETE FROM release WHERE repository_id = $1 AND workspace_id = $2;

-- name: DeleteReleasesByProject :exec
-- Releases reference their repository, never the project, so project delete
-- reaches them through repository; it runs before DeleteRepositoriesByProject.
DELETE FROM release
WHERE release.workspace_id = sqlc.arg('workspace_id')
  AND release.repository_id IN (
    SELECT repository.id FROM repository
    WHERE repository.project_id = sqlc.arg('project_id')
      AND repository.workspace_id = sqlc.arg('workspace_id')
  );

-- name: ListUnshippedReleaseHeads :many
-- The unshipped releases of a repository, named in the 409
-- has_unshipped_releases refusal of a repository that stops releasing.
SELECT version, title FROM release
WHERE workspace_id = $1 AND repository_id = $2 AND shipped_at IS NULL
ORDER BY position ASC;

-- name: LockReleaseLineOwners :one
-- Fence workspace/project teardown before taking the repository lock.
SELECT repository.id FROM repository
JOIN project ON project.id = repository.project_id AND project.workspace_id = repository.workspace_id
JOIN workspace ON workspace.id = repository.workspace_id
WHERE repository.id = $1 AND repository.workspace_id = $2
FOR KEY SHARE OF workspace, project;

-- name: MarkReleaseShipped :one
UPDATE release SET shipped_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND shipped_at IS NULL
RETURNING *;

-- name: LockReleaseIssuesForShipping :many
SELECT id FROM issue
WHERE workspace_id = $1 AND release_id = $2
ORDER BY id
FOR UPDATE;

-- name: ListReleaseShippingBlockers :many
-- A manual ship records an external publication. Every issue must be Done
-- and every linked pull request must be merged before that record is made.
SELECT i.id, (w.issue_prefix || '-' || i.number)::text AS identifier
FROM issue i JOIN workspace w ON w.id = i.workspace_id
WHERE i.workspace_id = $1 AND i.release_id = $2
  AND (
    issue_effective_status(i.workspace_id, i.status) <> 'done'
    OR EXISTS (
      SELECT 1 FROM issue_pull_request link
      LEFT JOIN github_pull_request pr ON pr.id = link.pull_request_id AND pr.workspace_id = i.workspace_id
      WHERE link.issue_id = i.id AND (pr.id IS NULL OR pr.state <> 'merged')
    )
    OR EXISTS (
      SELECT 1 FROM issue_vcs_pull_request link
      LEFT JOIN vcs_pull_request pr ON pr.id = link.pull_request_id AND pr.workspace_id = i.workspace_id
      WHERE link.issue_id = i.id AND (pr.id IS NULL OR pr.state <> 'merged')
    )
  )
ORDER BY i.number, i.id;

-- name: GetReleaseInWorkspaceForUpdate :one
-- Removal locks the release row right after the repository, before it deletes
-- the release's waits, discussion and runs. A release comment insert holds the
-- release FOR KEY SHARE before its capture key-shares the release rule, so
-- taking the release first keeps removal from waiting on a rule that insert
-- holds while the insert waits on the release.
SELECT * FROM release
WHERE id = $1 AND workspace_id = $2
FOR UPDATE;

-- name: UpdateReleaseOwner :one
-- Runs under LockReleaseLineOwners + LockRepositoryForLine. Both NULL removes
-- the owner; release_owner_pair_check refuses a partial or unknown pair.
UPDATE release SET
    owner_type = sqlc.narg('owner_type'),
    owner_id = sqlc.narg('owner_id'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: ListReleaseOwnerProjections :many
-- Owner name and avatar for a batch of releases, resolved in the release's
-- workspace only. A deleted owner keeps its type/id with a NULL name until
-- the owner cleanup clears it.
SELECT r.id AS release_id, r.owner_type, r.owner_id,
       COALESCE(a.name, s.name)::text AS owner_name,
       COALESCE(a.avatar_url, s.avatar_url) AS owner_avatar_url
FROM release r
LEFT JOIN agent a ON r.owner_type = 'agent' AND a.id = r.owner_id AND a.workspace_id = r.workspace_id
LEFT JOIN squad s ON r.owner_type = 'squad' AND s.id = r.owner_id AND s.workspace_id = r.workspace_id
WHERE r.workspace_id = sqlc.arg('workspace_id')
  AND r.id = ANY(sqlc.arg('release_ids')::uuid[])
  AND r.owner_id IS NOT NULL;

-- name: ListReleaseOwnerActivity :many
-- The release page's owner history: every repository-wide owner change of the
-- release, oldest first. No worklane creation cutoff applies here.
SELECT * FROM activity_log
WHERE workspace_id = sqlc.arg('workspace_id')
  AND repository_id = sqlc.arg('repository_id')
  AND worklane_id IS NULL
  AND action = 'release_owner_changed'
  AND details->>'release_id' = sqlc.arg('release_id')::uuid::text
ORDER BY created_at ASC, id ASC;

-- name: ListReleaseIssues :many
-- The one ordered issue projection of a release, read by the release page and
-- the coordination claim. Lanes follow the repository's worklane order (main
-- first, then position, name, id); within a lane lane_position, created_at, id.
-- Issues without a lane of this repository, including terminal issues whose
-- lane was removed, follow in created_at, id order. A read of existing order,
-- not a new rank.
SELECT i.id, i.number, (ws.issue_prefix || '-' || i.number)::text AS identifier,
       i.title, i.status,
       issue_effective_status(i.workspace_id, i.status)::text AS effective_status,
       (CASE
           WHEN i.status IN ('backlog', 'todo') THEN 'unstarted'
           WHEN i.status IN ('in_progress', 'in_review', 'blocked') THEN 'started'
           WHEN i.status = 'done' THEN 'done'
           WHEN i.status = 'cancelled' THEN 'closed'
           ELSE COALESCE(st.category, 'unstarted')
       END)::text AS status_category,
       i.assignee_type, i.assignee_id,
       COALESCE(aa.name, asq.name, au.name, '')::text AS assignee_name,
       COALESCE(aa.avatar_url, asq.avatar_url, au.avatar_url) AS assignee_avatar_url,
       i.parent_issue_id,
       COALESCE(ws.issue_prefix || '-' || parent.number, '')::text AS parent_identifier,
       parent.title AS parent_title,
       lane.id AS worklane_id, lane.name AS worklane_name, lane.is_main AS worklane_is_main,
       i.lane_position, i.created_at, i.updated_at
FROM release r
JOIN issue i ON i.release_id = r.id AND i.workspace_id = r.workspace_id
JOIN workspace ws ON ws.id = i.workspace_id
LEFT JOIN issue_status st ON st.workspace_id = i.workspace_id AND st.key = i.status
LEFT JOIN worklane lane ON lane.id = i.worklane_id AND lane.workspace_id = i.workspace_id AND lane.repository_id = r.repository_id
LEFT JOIN issue parent ON parent.id = i.parent_issue_id AND parent.workspace_id = i.workspace_id
LEFT JOIN agent aa ON i.assignee_type = 'agent' AND aa.id = i.assignee_id AND aa.workspace_id = i.workspace_id
LEFT JOIN squad asq ON i.assignee_type = 'squad' AND asq.id = i.assignee_id AND asq.workspace_id = i.workspace_id
LEFT JOIN "user" au ON i.assignee_type = 'member' AND au.id = i.assignee_id
    AND EXISTS (SELECT 1 FROM member m WHERE m.user_id = au.id AND m.workspace_id = i.workspace_id)
WHERE r.id = sqlc.arg('release_id') AND r.workspace_id = sqlc.arg('workspace_id')
ORDER BY (lane.id IS NULL) ASC, lane.is_main DESC, lane.position ASC, LOWER(lane.name) ASC, lane.id ASC,
         i.lane_position ASC NULLS LAST, i.created_at ASC, i.id ASC;

-- name: ListReleaseIssuePullRequests :many
-- GitHub and VCS pull requests linked to the issues of a release, in the
-- release's workspace.
SELECT link.issue_id, pr.id, 'github'::text AS provider, pr.pr_number AS number,
       pr.title, pr.state, pr.html_url AS url, pr.branch, pr.merged_at
FROM issue i
JOIN issue_pull_request link ON link.issue_id = i.id
JOIN github_pull_request pr ON pr.id = link.pull_request_id AND pr.workspace_id = i.workspace_id
WHERE i.workspace_id = sqlc.arg('workspace_id') AND i.release_id = sqlc.arg('release_id')
UNION ALL
SELECT link.issue_id, pr.id, pr.provider, pr.pr_number AS number,
       pr.title, pr.state, pr.html_url AS url, pr.branch, pr.merged_at
FROM issue i
JOIN issue_vcs_pull_request link ON link.issue_id = i.id
JOIN vcs_pull_request pr ON pr.id = link.pull_request_id AND pr.workspace_id = i.workspace_id
WHERE i.workspace_id = sqlc.arg('workspace_id') AND i.release_id = sqlc.arg('release_id')
ORDER BY 1, 3, 4, 2;

-- name: GetReleaseForTask :one
-- The release of a coordination run, fenced by the run's agent workspace so a
-- task cannot name another tenant's release. No row: the release is gone.
SELECT r.* FROM agent_task_queue t
JOIN agent a ON a.id = t.agent_id
JOIN release r ON r.id = t.release_id AND r.workspace_id = a.workspace_id
WHERE t.id = sqlc.arg('task_id');

-- name: LockReleaseTaskRepository :one
-- Claim finalize, start and completion of a coordination run take the
-- release-line boundary: workspace and project FOR KEY SHARE first, then the
-- repository FOR UPDATE, before lock_task_owner_rows and the task row. No row:
-- the release, its repository or their owners are gone.
WITH target AS MATERIALIZED (
    SELECT r.repository_id, r.workspace_id FROM agent_task_queue t
    JOIN agent a ON a.id = t.agent_id
    JOIN release r ON r.id = t.release_id AND r.workspace_id = a.workspace_id
    WHERE t.id = sqlc.arg('task_id')
), owner_fence AS MATERIALIZED (
    SELECT count(*) AS locked_count FROM (
        SELECT repository.id FROM target
        JOIN repository ON repository.id = target.repository_id AND repository.workspace_id = target.workspace_id
        JOIN project ON project.id = repository.project_id AND project.workspace_id = repository.workspace_id
        JOIN workspace ON workspace.id = repository.workspace_id
        FOR KEY SHARE OF workspace, project
    ) fenced
)
SELECT repository.* FROM repository
JOIN target ON repository.id = target.repository_id AND repository.workspace_id = target.workspace_id
CROSS JOIN owner_fence
WHERE owner_fence.locked_count > 0
FOR UPDATE OF repository;

-- name: LockReleaseOwnerSquad :one
-- Under the repository boundary, freezes a squad owner's leader for claim,
-- start and dispatch: a leader change waits for that transaction.
SELECT * FROM squad
WHERE id = $1 AND workspace_id = $2
FOR SHARE;

-- name: LockReleaseOwnerAgent :one
-- Under the repository boundary (after the squad), freezes the target agent's
-- archive state and runtime binding for claim, start and dispatch.
SELECT * FROM agent
WHERE id = $1 AND workspace_id = $2
FOR SHARE;

-- name: ListReleaseIDsByRepository :many
SELECT id FROM release
WHERE workspace_id = $1 AND repository_id = $2
ORDER BY id;

-- name: ListReleaseIDsByProject :many
SELECT release.id FROM release
JOIN repository ON repository.id = release.repository_id AND repository.workspace_id = release.workspace_id
WHERE release.workspace_id = sqlc.arg('workspace_id') AND repository.project_id = sqlc.arg('project_id')
ORDER BY release.id;

-- name: ListUnshippedReleasesOwnedBy :many
-- Releases a deleted owner leaves behind; each is re-read under its line locks.
SELECT * FROM release
WHERE workspace_id = $1 AND owner_type = $2 AND owner_id = $3 AND shipped_at IS NULL
ORDER BY id;

-- name: GetReleaseOwnerOriginator :one
-- The principal of the latest owner assignment of a release, read from its
-- owner history: the recorded originator, else the member who changed the
-- owner. An automatic coordination run invokes the owner as this person, not
-- as the release creator or the actor of a triggering issue change. NULL when
-- the release has no owner history.
SELECT (COALESCE(
    NULLIF(details->>'originator_user_id', ''),
    CASE WHEN actor_type = 'member' THEN actor_id::text END
))::uuid AS originator_user_id
FROM activity_log
WHERE workspace_id = sqlc.arg(workspace_id)
  AND worklane_id IS NULL
  AND action = 'release_owner_changed'
  AND details->>'release_id' = sqlc.arg(release_id)::uuid::text
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListReleaseTaskLockTargetsByAgent :many
-- The agent's active coordination runs in repository, release, task order, so
-- an agent-wide cancellation can take each run's repository boundary
-- (LockReleaseTaskRepository) in one stable order before cancelling.
SELECT t.id AS task_id FROM agent_task_queue t
JOIN agent a ON a.id = t.agent_id
JOIN release r ON r.id = t.release_id AND r.workspace_id = a.workspace_id
WHERE t.agent_id = @agent_id
  AND t.release_id IS NOT NULL
  AND t.status IN ('queued', 'dispatched', 'running', 'waiting_local_directory', 'deferred')
ORDER BY r.repository_id, r.id, t.id;
