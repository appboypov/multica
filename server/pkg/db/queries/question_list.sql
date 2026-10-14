-- Viewer row projections (ADR-0006). For me and its unfiltered count repeat
-- ADR-0004: open, a current answerer, and no answer from that member.
-- Filters accept repeated values; NULL or empty arrays leave a filter unset.
-- Only blocks_work = true filters. Cursor time and id are supplied together.

-- name: ListQuestionRowsForMember :many
WITH selected AS (
    SELECT q.id, q.ask_id, q.ask_position, q.ask_title, q.prompt,
           q.answer_type, q.settings, q.status, q.answer_policy,
           q.asker_agent_id, q.due_date, q.blocks_work, q.created_at,
           l.link_type, l.link_id,
           a.value AS my_answer_value, a.updated_at AS my_answer_updated_at
    FROM question q
    JOIN question_link l
      ON l.question_id = q.id
     AND l.workspace_id = q.workspace_id
     AND l.link_type = 'issue'
     AND l.position = 0
    LEFT JOIN question_answer a
      ON a.question_id = q.id
     AND a.workspace_id = q.workspace_id
     AND a.answerer_user_id = sqlc.arg('viewer_id')::uuid
    WHERE q.workspace_id = sqlc.arg('workspace_id')::uuid
      AND q.status IN ('open', 'partly_answered')
      AND EXISTS (
          SELECT 1 FROM question_answerer r
          WHERE r.question_id = q.id
            AND r.workspace_id = q.workspace_id
            AND r.user_id = sqlc.arg('viewer_id')::uuid
      )
      AND a.id IS NULL
      AND (COALESCE(cardinality(sqlc.narg('answer_type')::text[]), 0) = 0
           OR q.answer_type = ANY(sqlc.narg('answer_type')::text[]))
      AND (COALESCE(cardinality(sqlc.narg('asker_agent_id')::uuid[]), 0) = 0
           OR q.asker_agent_id = ANY(sqlc.narg('asker_agent_id')::uuid[]))
      AND (COALESCE(cardinality(sqlc.narg('answerer_user_id')::uuid[]), 0) = 0
           OR EXISTS (
               SELECT 1 FROM question_answerer r
               WHERE r.question_id = q.id
                 AND r.workspace_id = q.workspace_id
                 AND r.user_id = ANY(sqlc.narg('answerer_user_id')::uuid[])
           ))
      AND (COALESCE(cardinality(sqlc.narg('link_ids')::uuid[]), 0) = 0
           OR l.link_id = ANY(sqlc.narg('link_ids')::uuid[]))
      AND (sqlc.narg('blocks_work')::boolean IS NULL
           OR NOT sqlc.narg('blocks_work')::boolean OR q.blocks_work)
      AND (sqlc.narg('cursor_time')::timestamptz IS NULL
           OR (q.created_at, q.id) >
              (sqlc.narg('cursor_time')::timestamptz, sqlc.narg('cursor_id')::uuid))
    ORDER BY q.created_at ASC, q.id ASC
    LIMIT sqlc.arg('limit')::int
)
SELECT selected.*,
       ARRAY(
           SELECT r.user_id FROM question_answerer r
           WHERE r.workspace_id = sqlc.arg('workspace_id')::uuid
             AND r.question_id = selected.id
           ORDER BY r.assigned_at ASC, r.user_id ASC
       )::uuid[] AS answerer_ids
FROM selected
ORDER BY selected.created_at ASC, selected.id ASC;

-- name: ListOpenQuestionRows :many
WITH selected AS (
    SELECT q.id, q.ask_id, q.ask_position, q.ask_title, q.prompt,
           q.answer_type, q.settings, q.status, q.answer_policy,
           q.asker_agent_id, q.due_date, q.blocks_work, q.created_at,
           l.link_type, l.link_id,
           a.value AS my_answer_value, a.updated_at AS my_answer_updated_at
    FROM question q
    JOIN question_link l
      ON l.question_id = q.id
     AND l.workspace_id = q.workspace_id
     AND l.link_type = 'issue'
     AND l.position = 0
    LEFT JOIN question_answer a
      ON a.question_id = q.id
     AND a.workspace_id = q.workspace_id
     AND a.answerer_user_id = sqlc.arg('viewer_id')::uuid
    WHERE q.workspace_id = sqlc.arg('workspace_id')::uuid
      AND q.status IN ('open', 'partly_answered')
      AND (COALESCE(cardinality(sqlc.narg('answer_type')::text[]), 0) = 0
           OR q.answer_type = ANY(sqlc.narg('answer_type')::text[]))
      AND (COALESCE(cardinality(sqlc.narg('asker_agent_id')::uuid[]), 0) = 0
           OR q.asker_agent_id = ANY(sqlc.narg('asker_agent_id')::uuid[]))
      AND (COALESCE(cardinality(sqlc.narg('answerer_user_id')::uuid[]), 0) = 0
           OR EXISTS (
               SELECT 1 FROM question_answerer r
               WHERE r.question_id = q.id
                 AND r.workspace_id = q.workspace_id
                 AND r.user_id = ANY(sqlc.narg('answerer_user_id')::uuid[])
           ))
      AND (COALESCE(cardinality(sqlc.narg('link_ids')::uuid[]), 0) = 0
           OR l.link_id = ANY(sqlc.narg('link_ids')::uuid[]))
      AND (sqlc.narg('blocks_work')::boolean IS NULL
           OR NOT sqlc.narg('blocks_work')::boolean OR q.blocks_work)
      AND (sqlc.narg('cursor_time')::timestamptz IS NULL
           OR (q.created_at, q.id) >
              (sqlc.narg('cursor_time')::timestamptz, sqlc.narg('cursor_id')::uuid))
    ORDER BY q.created_at ASC, q.id ASC
    LIMIT sqlc.arg('limit')::int
)
SELECT selected.*,
       ARRAY(
           SELECT r.user_id FROM question_answerer r
           WHERE r.workspace_id = sqlc.arg('workspace_id')::uuid
             AND r.question_id = selected.id
           ORDER BY r.assigned_at ASC, r.user_id ASC
       )::uuid[] AS answerer_ids
FROM selected
ORDER BY selected.created_at ASC, selected.id ASC;

-- name: ListQuestionRowsAnsweredBy :many
WITH selected AS (
    SELECT q.id, q.ask_id, q.ask_position, q.ask_title, q.prompt,
           q.answer_type, q.settings, q.status, q.answer_policy,
           q.asker_agent_id, q.due_date, q.blocks_work, q.created_at,
           l.link_type, l.link_id,
           a.value AS my_answer_value, a.updated_at AS my_answer_updated_at
    FROM question_answer a
    JOIN question q
      ON q.id = a.question_id
     AND q.workspace_id = a.workspace_id
    JOIN question_link l
      ON l.question_id = q.id
     AND l.workspace_id = q.workspace_id
     AND l.link_type = 'issue'
     AND l.position = 0
    WHERE a.workspace_id = sqlc.arg('workspace_id')::uuid
      AND a.answerer_user_id = sqlc.arg('viewer_id')::uuid
      AND (COALESCE(cardinality(sqlc.narg('answer_type')::text[]), 0) = 0
           OR q.answer_type = ANY(sqlc.narg('answer_type')::text[]))
      AND (COALESCE(cardinality(sqlc.narg('asker_agent_id')::uuid[]), 0) = 0
           OR q.asker_agent_id = ANY(sqlc.narg('asker_agent_id')::uuid[]))
      AND (COALESCE(cardinality(sqlc.narg('answerer_user_id')::uuid[]), 0) = 0
           OR EXISTS (
               SELECT 1 FROM question_answerer r
               WHERE r.question_id = q.id
                 AND r.workspace_id = q.workspace_id
                 AND r.user_id = ANY(sqlc.narg('answerer_user_id')::uuid[])
           ))
      AND (COALESCE(cardinality(sqlc.narg('link_ids')::uuid[]), 0) = 0
           OR l.link_id = ANY(sqlc.narg('link_ids')::uuid[]))
      AND (sqlc.narg('blocks_work')::boolean IS NULL
           OR NOT sqlc.narg('blocks_work')::boolean OR q.blocks_work)
      AND (sqlc.narg('cursor_time')::timestamptz IS NULL
           OR (a.updated_at, a.question_id) <
              (sqlc.narg('cursor_time')::timestamptz, sqlc.narg('cursor_id')::uuid))
    ORDER BY a.updated_at DESC, a.question_id DESC
    LIMIT sqlc.arg('limit')::int
)
SELECT selected.*,
       ARRAY(
           SELECT r.user_id FROM question_answerer r
           WHERE r.workspace_id = sqlc.arg('workspace_id')::uuid
             AND r.question_id = selected.id
           ORDER BY r.assigned_at ASC, r.user_id ASC
       )::uuid[] AS answerer_ids
FROM selected
ORDER BY selected.my_answer_updated_at DESC, selected.id DESC;

-- name: CountQuestionsOpenForMember :one
-- The sidebar counts all questions open for the member, without list filters.
SELECT count(*)::bigint AS count
FROM question q
WHERE q.workspace_id = sqlc.arg('workspace_id')::uuid
  AND q.status IN ('open', 'partly_answered')
  AND EXISTS (
      SELECT 1 FROM question_answerer r
      WHERE r.question_id = q.id
        AND r.workspace_id = q.workspace_id
        AND r.user_id = sqlc.arg('viewer_id')::uuid
  )
  AND NOT EXISTS (
      SELECT 1 FROM question_answer a
      WHERE a.question_id = q.id
        AND a.workspace_id = q.workspace_id
        AND a.answerer_user_id = sqlc.arg('viewer_id')::uuid
  );
