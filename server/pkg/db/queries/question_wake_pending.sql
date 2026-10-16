-- name: UpsertQuestionWakePending :one
-- Caller holds the issue lock; coalescing retains the first accepted attribution.
INSERT INTO question_wake_pending (
    workspace_id, issue_id, agent_id, actor_user_id, source_question_id, answer_anchor
) VALUES (@workspace_id, @issue_id, @agent_id, @actor_user_id, @source_question_id, @answer_anchor)
ON CONFLICT (workspace_id, issue_id, agent_id) DO UPDATE SET
    answer_anchor = LEAST(question_wake_pending.answer_anchor, EXCLUDED.answer_anchor)
RETURNING *;

-- name: GetQuestionWakePending :one
SELECT * FROM question_wake_pending
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND agent_id = @agent_id;

-- name: GetQuestionWakePendingForUpdate :one
-- Lock after the issue and before the source question.
SELECT * FROM question_wake_pending
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND agent_id = @agent_id
FOR UPDATE;

-- name: DeleteQuestionWakePending :exec
DELETE FROM question_wake_pending
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND agent_id = @agent_id;

-- name: ListQuestionWakePending :many
-- Recovery intentionally spans workspaces, including waits whose targets vanished.
SELECT * FROM question_wake_pending
WHERE sqlc.narg('after_created_at')::timestamptz IS NULL
   OR (created_at, workspace_id, issue_id, agent_id) >
      (sqlc.narg('after_created_at')::timestamptz, @after_workspace_id::uuid,
       @after_issue_id::uuid, @after_agent_id::uuid)
ORDER BY created_at, workspace_id, issue_id, agent_id
LIMIT @max_rows;

-- name: GetQuestionWakeAnswerAnchor :one
SELECT question_wake_answer_cutoff(@workspace_id::uuid, @issue_id::uuid, @agent_id::uuid,
                                  NULL::uuid, NULL::timestamptz)::timestamptz AS answer_anchor;

-- name: ListQuestionWakeRetryCandidates :many
-- Only failed tails that ended during this durable wait can have an outstanding
-- retry decision. The caller applies the existing retry policy under a parent lock.
SELECT t.* FROM agent_task_queue t
JOIN agent a ON a.id = t.agent_id AND a.workspace_id = @workspace_id
JOIN issue i ON i.id = t.issue_id AND i.workspace_id = @workspace_id
JOIN question_wake_pending pending ON pending.workspace_id = @workspace_id
  AND pending.issue_id = t.issue_id AND pending.agent_id = t.agent_id
WHERE t.agent_id = @agent_id AND t.issue_id = @issue_id
  AND t.status = 'failed' AND t.completed_at >= pending.created_at
  AND NOT EXISTS (
      SELECT 1 FROM agent_task_queue child WHERE child.retry_of_task_id = t.id
  )
  AND NOT EXISTS (
      SELECT 1 FROM agent_task_queue successor
      WHERE successor.agent_id = t.agent_id AND successor.issue_id = t.issue_id
        AND successor.comment_thread_id IS NOT DISTINCT FROM t.comment_thread_id
        AND (successor.created_at, successor.id) > (t.created_at, t.id)
  )
ORDER BY t.completed_at, t.id;

-- name: LockQuestionWakeTransition :exec
-- Shared holders fence delivery while allowing synchronous nested transitions.
SELECT pg_advisory_xact_lock_shared(hashtextextended((sqlc.arg(issue_id)::uuid)::text || ':question_wake', 0));

-- name: TryLockQuestionWakeDelivery :one
-- Delivery never waits on an enclosing transition or another delivery.
SELECT pg_try_advisory_xact_lock(hashtextextended((sqlc.arg(issue_id)::uuid)::text || ':question_wake', 0))::boolean AS acquired;

-- name: ListQuestionWakePendingForIssues :many
SELECT * FROM question_wake_pending
WHERE workspace_id = @workspace_id AND issue_id = ANY(@issue_ids::uuid[]);
