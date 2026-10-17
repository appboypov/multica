-- name: OpenTodo :one
-- Only a current member of the workspace gets a to-do.
INSERT INTO todo (
    id, workspace_id, member_id, kind, source_key, issue_id, question_id,
    comment_id, task_id, autopilot_id, actor_type, actor_id, title, body, details
)
SELECT
    @id::uuid, @workspace_id::uuid, @member_id::uuid, @kind::text, @source_key::text,
    sqlc.narg('issue_id')::uuid, sqlc.narg('question_id')::uuid,
    sqlc.narg('comment_id')::uuid, sqlc.narg('task_id')::uuid,
    sqlc.narg('autopilot_id')::uuid, sqlc.narg('actor_type')::text,
    sqlc.narg('actor_id')::uuid, @title::text, sqlc.narg('body')::text, @details::jsonb
WHERE EXISTS (
    SELECT 1 FROM member m
    WHERE m.workspace_id = @workspace_id::uuid AND m.user_id = @member_id::uuid
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

-- name: MoveTodosFromDeletedIssue :many
-- An open to-do whose issue is gone follows its source to the issue it still
-- reaches, or to none. Done to-dos keep their issue.
UPDATE todo t
SET issue_id = sqlc.narg('issue_id')::uuid, title = @title
WHERE t.workspace_id = @workspace_id AND t.kind = @kind AND t.source_key = @source_key
  AND t.done_at IS NULL AND t.issue_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM issue i WHERE i.id = t.issue_id AND i.workspace_id = t.workspace_id)
RETURNING t.*;

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

-- name: LockTodoReviewVisibility :exec
-- Serializes the writes that decide whether a member's review on an issue is visible.
SELECT pg_advisory_xact_lock(hashtextextended(
    sqlc.arg('workspace_id')::uuid::text || ':' || sqlc.arg('member_id')::uuid::text || ':'
        || sqlc.arg('issue_id')::uuid::text || ':todo_review', 0
));

-- name: ListTodoVisibilityChanges :many
-- Reconstruct visibility before this write, including reviews affected by questions.
WITH changed AS (
    SELECT * FROM todo WHERE id = ANY(@ids::uuid[])
), affected AS (
    SELECT id FROM changed
    UNION
    SELECT t.id
    FROM changed c
    JOIN todo t ON t.workspace_id = c.workspace_id
               AND t.member_id = c.member_id AND t.issue_id = c.issue_id
    WHERE c.kind = 'question' AND t.kind = 'review' AND t.done_at IS NULL
), candidates AS (
    SELECT t.*
    FROM affected a
    JOIN todo t ON t.id = a.id
    LEFT JOIN issue i ON i.id = t.issue_id AND i.workspace_id = t.workspace_id
    WHERE t.issue_id IS NULL OR i.id IS NOT NULL
), visibility AS (
    SELECT t.*,
        ((CASE WHEN t.id = ANY(@ids::uuid[]) THEN NOT @opening::boolean
               ELSE t.done_at IS NULL END)
         AND (t.kind <> 'review' OR NOT EXISTS (
             SELECT 1 FROM todo q
             WHERE q.workspace_id = t.workspace_id AND q.member_id = t.member_id
               AND q.issue_id = t.issue_id AND q.kind = 'question'
               AND CASE WHEN q.id = ANY(@ids::uuid[]) THEN NOT @opening::boolean
                        ELSE q.done_at IS NULL END
         )))::boolean AS was_visible,
        (t.done_at IS NULL AND (t.kind <> 'review' OR NOT EXISTS (
            SELECT 1 FROM todo q
            WHERE q.workspace_id = t.workspace_id AND q.member_id = t.member_id
              AND q.issue_id = t.issue_id AND q.kind = 'question'
              AND q.done_at IS NULL
        )))::boolean AS is_visible
    FROM candidates t
)
SELECT id, workspace_id, member_id, kind, done_by, is_visible
FROM visibility
WHERE was_visible <> is_visible
ORDER BY is_visible, id;
