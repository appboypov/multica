-- name: OpenTodo :one
INSERT INTO todo (
    id, workspace_id, member_id, kind, source_key, issue_id, question_id,
    comment_id, task_id, autopilot_id, actor_type, actor_id, title, body, details
) VALUES (
    @id, @workspace_id, @member_id, @kind, @source_key,
    sqlc.narg('issue_id')::uuid, sqlc.narg('question_id')::uuid,
    sqlc.narg('comment_id')::uuid, sqlc.narg('task_id')::uuid,
    sqlc.narg('autopilot_id')::uuid, sqlc.narg('actor_type')::text,
    sqlc.narg('actor_id')::uuid, @title, sqlc.narg('body')::text, @details
)
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL
DO NOTHING
RETURNING *;

-- name: CloseTodos :many
-- A NULL member closes the source for all of its recipients.
UPDATE todo
SET done_at = now(), done_by = 'handled'
WHERE workspace_id = @workspace_id AND kind = @kind AND source_key = @source_key
  AND (sqlc.narg('member_id')::uuid IS NULL OR member_id = sqlc.narg('member_id')::uuid)
  AND done_at IS NULL
RETURNING *;

-- name: CloseTodosOnIssue :many
UPDATE todo
SET done_at = now(), done_by = 'handled'
WHERE workspace_id = @workspace_id AND issue_id = @issue_id
  AND kind = ANY(sqlc.arg('kinds')::text[])
  AND (sqlc.narg('member_id')::uuid IS NULL OR member_id = sqlc.narg('member_id')::uuid)
  AND done_at IS NULL
RETURNING *;

-- name: CloseTodosInWorkspace :many
UPDATE todo
SET done_at = now(), done_by = 'handled'
WHERE workspace_id = @workspace_id AND kind = @kind AND done_at IS NULL
RETURNING *;

-- name: ListOpenTodos :many
WITH issue_projection AS (
    SELECT i.id, i.workspace_id, i.title, i.status, i.due_date,
           (w.issue_prefix || '-' || i.number)::text AS identifier
    FROM issue i
    JOIN workspace w ON w.id = i.workspace_id
)
SELECT t.id, t.workspace_id, t.member_id, t.kind, t.source_key,
       t.issue_id, t.question_id, t.comment_id, t.task_id, t.autopilot_id,
       t.actor_type, t.actor_id, COALESCE(i.title, t.title)::text AS title,
       t.body, t.details, t.created_at, t.done_at, t.done_by,
       i.identifier AS issue_identifier,
       i.title AS issue_title, i.status AS issue_status,
       q.answer_type AS question_answer_type,
       (CASE WHEN t.kind = 'question' THEN q.due_date
             WHEN t.kind IN ('assigned', 'review') THEN i.due_date END)::date AS due_date,
       COALESCE((CASE WHEN t.kind = 'question' THEN q.due_date
                      WHEN t.kind IN ('assigned', 'review') THEN i.due_date END) < CURRENT_DATE, false)::boolean AS overdue
FROM todo t
JOIN workspace w ON w.id = t.workspace_id
LEFT JOIN issue_projection i ON i.id = t.issue_id AND i.workspace_id = t.workspace_id
LEFT JOIN question q ON q.id = t.question_id AND q.workspace_id = t.workspace_id
WHERE t.workspace_id = @workspace_id AND t.member_id = @member_id
  AND t.done_at IS NULL
  AND (t.issue_id IS NULL OR i.id IS NOT NULL)
  AND (t.kind <> 'review' OR NOT EXISTS (
      SELECT 1 FROM todo question_todo
      WHERE question_todo.workspace_id = t.workspace_id
        AND question_todo.member_id = t.member_id
        AND question_todo.issue_id = t.issue_id
        AND question_todo.kind = 'question' AND question_todo.done_at IS NULL
  ))
ORDER BY overdue DESC, t.created_at DESC, t.id DESC;

-- name: CountOpenTodosByWorkspace :many
SELECT t.workspace_id, count(*) AS count
FROM todo t
LEFT JOIN issue i ON i.id = t.issue_id AND i.workspace_id = t.workspace_id
WHERE t.member_id = @member_id AND t.done_at IS NULL
  AND EXISTS (
      SELECT 1 FROM member m
      WHERE m.workspace_id = t.workspace_id AND m.user_id = t.member_id
  )
  AND (t.issue_id IS NULL OR i.id IS NOT NULL)
  AND (t.kind <> 'review' OR NOT EXISTS (
      SELECT 1 FROM todo question_todo
      WHERE question_todo.workspace_id = t.workspace_id
        AND question_todo.member_id = t.member_id
        AND question_todo.issue_id = t.issue_id
        AND question_todo.kind = 'question' AND question_todo.done_at IS NULL
  ))
GROUP BY t.workspace_id
ORDER BY t.workspace_id;

-- name: GetTodoForMember :one
WITH issue_projection AS (
    SELECT i.id, i.workspace_id, i.title, i.status, i.due_date,
           (w.issue_prefix || '-' || i.number)::text AS identifier
    FROM issue i
    JOIN workspace w ON w.id = i.workspace_id
)
SELECT t.id, t.workspace_id, t.member_id, t.kind, t.source_key,
       t.issue_id, t.question_id, t.comment_id, t.task_id, t.autopilot_id,
       t.actor_type, t.actor_id, COALESCE(i.title, t.title)::text AS title,
       t.body, t.details, t.created_at, t.done_at, t.done_by,
       i.identifier AS issue_identifier,
       i.title AS issue_title, i.status AS issue_status,
       q.answer_type AS question_answer_type,
       (CASE WHEN t.kind = 'question' THEN q.due_date
             WHEN t.kind IN ('assigned', 'review') THEN i.due_date END)::date AS due_date,
       COALESCE((CASE WHEN t.kind = 'question' THEN q.due_date
                      WHEN t.kind IN ('assigned', 'review') THEN i.due_date END) < CURRENT_DATE, false)::boolean AS overdue
FROM todo t
JOIN workspace w ON w.id = t.workspace_id
LEFT JOIN issue_projection i ON i.id = t.issue_id AND i.workspace_id = t.workspace_id
LEFT JOIN question q ON q.id = t.question_id AND q.workspace_id = t.workspace_id
WHERE t.id = @id AND t.workspace_id = @workspace_id AND t.member_id = @member_id
  AND (t.issue_id IS NULL OR i.id IS NOT NULL);

-- name: MarkTodoDone :one
UPDATE todo
SET done_at = now(), done_by = 'member'
WHERE id = @id AND workspace_id = @workspace_id AND member_id = @member_id
  AND done_at IS NULL
RETURNING *;
