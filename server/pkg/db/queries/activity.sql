-- name: ListActivitiesForIssue :many
-- The NEWEST $2 activities for an issue, returned in chronological order.
--
-- The cap has to bite at the OLD end, not the new one. The inner query takes
-- the window with the keyset ordering (created_at DESC, id DESC), which
-- idx_activity_log_issue_keyset (migration 068) satisfies without a sort step —
-- it is not an index-only scan, since the index does not cover the columns
-- SELECT * needs, so the heap is still read for the rows in the window. The
-- outer query re-sorts ascending so every caller keeps the chronological
-- contract it already had.
--
-- Capping with ORDER BY created_at ASC instead discards the newest rows, which
-- made a busy issue's timeline appear to stop at some point in the past with no
-- indication anything was missing. Activity is machine-paced (description
-- autosave, every agent run, status/assignee changes), so this was reachable in
-- normal use, not only on pathological issues (MUL-5492).
SELECT * FROM (
    SELECT * FROM activity_log
    WHERE issue_id = $1
    ORDER BY created_at DESC, id DESC
    LIMIT $2
) AS recent
ORDER BY created_at ASC, id ASC;

-- name: GetActivity :one
SELECT * FROM activity_log
WHERE id = $1;

-- name: CreateActivity :one
INSERT INTO activity_log (
    workspace_id, issue_id, actor_type, actor_id, action, details, id
) VALUES ($1, $2, $3, $4, $5, $6, COALESCE(sqlc.narg('id')::uuid, gen_random_uuid()))
RETURNING *;

-- name: HasSquadLeaderNoActionEvaluationForTask :one
SELECT EXISTS (
  SELECT 1
  FROM activity_log
  WHERE issue_id = @issue_id
    AND actor_type = 'agent'
    AND actor_id = @agent_id
    AND action = 'squad_leader_evaluated'
    AND details->>'outcome' = 'no_action'
    AND details->>'task_id' = @task_id::text
) AS exists;

-- name: CountAssigneeChangesByActor :many
-- Count how many times a user assigned each target via assignee_changed activities.
SELECT
  details->>'to_type' as assignee_type,
  details->>'to_id' as assignee_id,
  COUNT(*)::bigint as frequency
FROM activity_log
WHERE workspace_id = $1
  AND actor_id = $2
  AND actor_type = 'member'
  AND action = 'assignee_changed'
  AND details->>'to_type' IS NOT NULL
  AND details->>'to_id' IS NOT NULL
GROUP BY details->>'to_type', details->>'to_id';

-- name: CreateWorklaneActivity :one
-- A worklane entry (ADR-0008): issue_id NULL, the worklane and its repository
-- set, so deleting the repository or its project deletes it too.
INSERT INTO activity_log (
    workspace_id, worklane_id, repository_id, actor_type, actor_id, action, details
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('worklane_id'), sqlc.arg('repository_id'),
    sqlc.arg('actor_type'), sqlc.narg('actor_id'), sqlc.arg('action'),
    sqlc.arg('details')
)
RETURNING *;

-- name: CreateRepositoryActivity :one
-- A repository-wide entry, such as a release line write (ADR-0008): the
-- repository set, worklane_id and issue_id NULL. It shows on every worklane
-- of the repository made before it.
INSERT INTO activity_log (
    workspace_id, repository_id, actor_type, actor_id, action, details
) VALUES (
    sqlc.arg('workspace_id'), sqlc.arg('repository_id'), sqlc.arg('actor_type'),
    sqlc.narg('actor_id'), sqlc.arg('action'), sqlc.arg('details')
)
RETURNING *;

-- name: ListActivitiesForWorklane :many
-- The NEWEST 200 entries of a worklane, returned in chronological order, as
-- ListActivitiesForIssue: the worklane's own entries and its repository's
-- repository-wide entries (worklane_id NULL) from the worklane's creation on
-- (ADR-0011), which show on every worklane. worklane_created_at is the
-- worklane's created_at; main is made with its repository, so it shows every
-- entry. activity_log_worklane_keyset_idx and
-- activity_log_repository_keyset_idx serve the two sides.
SELECT * FROM (
    SELECT * FROM activity_log
    WHERE workspace_id = sqlc.arg('workspace_id')
      AND issue_id IS NULL
      AND (
        worklane_id = sqlc.arg('worklane_id')
        OR (
          repository_id = sqlc.arg('repository_id')
          AND worklane_id IS NULL
          AND created_at >= sqlc.arg('worklane_created_at')
        )
      )
    ORDER BY created_at DESC, id DESC
    LIMIT 200
) AS recent
ORDER BY created_at ASC, id ASC;

-- name: DeleteActivitiesByWorklane :exec
DELETE FROM activity_log
WHERE worklane_id = $1 AND workspace_id = $2 AND issue_id IS NULL;

-- name: DeleteActivitiesByRepository :exec
-- The repository's worklane entries and its repository-wide entries.
DELETE FROM activity_log
WHERE repository_id = $1 AND workspace_id = $2 AND issue_id IS NULL;

-- name: DeleteActivitiesByProject :exec
-- Activity references its repository, never the project, so project delete
-- reaches it through repository; it runs before DeleteRepositoriesByProject.
DELETE FROM activity_log
WHERE activity_log.workspace_id = sqlc.arg('workspace_id')
  AND activity_log.issue_id IS NULL
  AND activity_log.repository_id IN (
    SELECT repository.id FROM repository
    WHERE repository.project_id = sqlc.arg('project_id')
      AND repository.workspace_id = sqlc.arg('workspace_id')
  );
