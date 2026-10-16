-- Viewer row projections (ADR-0006). For me and its unfiltered count repeat
-- ADR-0004: open, a current answerer, and no answer from that member.
-- A row carries its first link, of any type; the service turns it into the
-- page object. Filters accept repeated values; NULL or empty arrays leave a
-- filter unset. Only blocks_work = true filters. Cursor time and id are
-- supplied together.

-- name: ListQuestionRowsForMember :many
WITH selected AS (
    SELECT q.id, q.ask_id, q.ask_position, q.ask_title, q.prompt,
           q.answer_type, q.settings, q.status, q.answer_policy,
           q.asker_agent_id, q.asker_user_id, q.due_date, q.blocks_work, q.created_at,
           l.link_type, l.link_id,
           a.value AS my_answer_value, a.updated_at AS my_answer_updated_at
    FROM question q
    JOIN question_link l
      ON l.question_id = q.id
     AND l.workspace_id = q.workspace_id
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
      AND (COALESCE(cardinality(sqlc.narg('asker_ids')::uuid[]), 0) = 0
           OR (q.asker_agent_id = ANY(sqlc.narg('asker_ids')::uuid[])
               OR q.asker_user_id = ANY(sqlc.narg('asker_ids')::uuid[])))
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
           q.asker_agent_id, q.asker_user_id, q.due_date, q.blocks_work, q.created_at,
           l.link_type, l.link_id,
           a.value AS my_answer_value, a.updated_at AS my_answer_updated_at
    FROM question q
    JOIN question_link l
      ON l.question_id = q.id
     AND l.workspace_id = q.workspace_id
     AND l.position = 0
    LEFT JOIN question_answer a
      ON a.question_id = q.id
     AND a.workspace_id = q.workspace_id
     AND a.answerer_user_id = sqlc.arg('viewer_id')::uuid
    WHERE q.workspace_id = sqlc.arg('workspace_id')::uuid
      AND q.status IN ('open', 'partly_answered')
      AND (COALESCE(cardinality(sqlc.narg('answer_type')::text[]), 0) = 0
           OR q.answer_type = ANY(sqlc.narg('answer_type')::text[]))
      AND (COALESCE(cardinality(sqlc.narg('asker_ids')::uuid[]), 0) = 0
           OR (q.asker_agent_id = ANY(sqlc.narg('asker_ids')::uuid[])
               OR q.asker_user_id = ANY(sqlc.narg('asker_ids')::uuid[])))
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
           q.asker_agent_id, q.asker_user_id, q.due_date, q.blocks_work, q.created_at,
           l.link_type, l.link_id,
           a.value AS my_answer_value, a.updated_at AS my_answer_updated_at
    FROM question_answer a
    JOIN question q
      ON q.id = a.question_id
     AND q.workspace_id = a.workspace_id
    JOIN question_link l
      ON l.question_id = q.id
     AND l.workspace_id = q.workspace_id
     AND l.position = 0
    WHERE a.workspace_id = sqlc.arg('workspace_id')::uuid
      AND a.answerer_user_id = sqlc.arg('viewer_id')::uuid
      AND (COALESCE(cardinality(sqlc.narg('answer_type')::text[]), 0) = 0
           OR q.answer_type = ANY(sqlc.narg('answer_type')::text[]))
      AND (COALESCE(cardinality(sqlc.narg('asker_ids')::uuid[]), 0) = 0
           OR (q.asker_agent_id = ANY(sqlc.narg('asker_ids')::uuid[])
               OR q.asker_user_id = ANY(sqlc.narg('asker_ids')::uuid[])))
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
SELECT count(*)::bigint AS count,
       count(*) FILTER (WHERE q.blocks_work)::bigint AS blocking_count
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

-- name: CountQuestionsOpen :one
SELECT count(*)::bigint AS count,
       count(*) FILTER (WHERE q.blocks_work)::bigint AS blocking_count
FROM question q
WHERE q.workspace_id = sqlc.arg('workspace_id')::uuid
  AND q.status IN ('open', 'partly_answered');

-- The downward form of a question's reach (ADR-0007): the open questions
-- below the seed issues and projects, the same set QuestionIDsBelow reads.
-- The seeds are the viewer's work for On my work, or one page object for parent.

-- name: ListQuestionRowsBelow :many
WITH RECURSIVE subtree AS (
    SELECT i.id FROM issue i
    WHERE i.workspace_id = sqlc.arg('workspace_id')::uuid
      AND (i.id = ANY(sqlc.arg('issue_ids')::uuid[])
           OR i.project_id = ANY(sqlc.arg('project_ids')::uuid[]))
    UNION
    SELECT child.id FROM issue child JOIN subtree parent ON child.parent_issue_id = parent.id
    WHERE child.workspace_id = sqlc.arg('workspace_id')::uuid
), below AS (
    SELECT l.question_id FROM question_link l
    WHERE l.workspace_id = sqlc.arg('workspace_id')::uuid AND (
        (l.link_type = 'issue' AND l.link_id IN (SELECT id FROM subtree))
        OR (l.link_type = 'comment' AND EXISTS (
            SELECT 1 FROM comment c
            WHERE c.id = l.link_id AND c.workspace_id = l.workspace_id
              AND c.deleted_at IS NULL AND c.issue_id IN (SELECT id FROM subtree)
        ))
        OR (l.link_type = 'task' AND EXISTS (
            SELECT 1 FROM agent_task_queue t
            JOIN agent ag ON ag.id = t.agent_id AND ag.workspace_id = l.workspace_id
            WHERE t.id = l.link_id AND t.issue_id IN (SELECT id FROM subtree)
        ))
        OR (l.link_type = 'project' AND l.link_id = ANY(sqlc.arg('project_ids')::uuid[]))
    )
), selected AS (
    SELECT q.id, q.ask_id, q.ask_position, q.ask_title, q.prompt,
           q.answer_type, q.settings, q.status, q.answer_policy,
           q.asker_agent_id, q.asker_user_id, q.due_date, q.blocks_work, q.created_at,
           l.link_type, l.link_id,
           a.value AS my_answer_value, a.updated_at AS my_answer_updated_at
    FROM question q
    JOIN question_link l
      ON l.question_id = q.id
     AND l.workspace_id = q.workspace_id
     AND l.position = 0
    LEFT JOIN question_answer a
      ON a.question_id = q.id
     AND a.workspace_id = q.workspace_id
     AND a.answerer_user_id = sqlc.arg('viewer_id')::uuid
    WHERE q.workspace_id = sqlc.arg('workspace_id')::uuid
      AND q.status IN ('open', 'partly_answered')
      AND q.id IN (SELECT question_id FROM below)
      AND (COALESCE(cardinality(sqlc.narg('answer_type')::text[]), 0) = 0
           OR q.answer_type = ANY(sqlc.narg('answer_type')::text[]))
      AND (COALESCE(cardinality(sqlc.narg('asker_ids')::uuid[]), 0) = 0
           OR (q.asker_agent_id = ANY(sqlc.narg('asker_ids')::uuid[])
               OR q.asker_user_id = ANY(sqlc.narg('asker_ids')::uuid[])))
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

-- name: CountQuestionsBelow :one
WITH RECURSIVE subtree AS (
    SELECT i.id FROM issue i
    WHERE i.workspace_id = sqlc.arg('workspace_id')::uuid
      AND (i.id = ANY(sqlc.arg('issue_ids')::uuid[])
           OR i.project_id = ANY(sqlc.arg('project_ids')::uuid[]))
    UNION
    SELECT child.id FROM issue child JOIN subtree parent ON child.parent_issue_id = parent.id
    WHERE child.workspace_id = sqlc.arg('workspace_id')::uuid
), below AS (
    SELECT l.question_id FROM question_link l
    WHERE l.workspace_id = sqlc.arg('workspace_id')::uuid AND (
        (l.link_type = 'issue' AND l.link_id IN (SELECT id FROM subtree))
        OR (l.link_type = 'comment' AND EXISTS (
            SELECT 1 FROM comment c
            WHERE c.id = l.link_id AND c.workspace_id = l.workspace_id
              AND c.deleted_at IS NULL AND c.issue_id IN (SELECT id FROM subtree)
        ))
        OR (l.link_type = 'task' AND EXISTS (
            SELECT 1 FROM agent_task_queue t
            JOIN agent ag ON ag.id = t.agent_id AND ag.workspace_id = l.workspace_id
            WHERE t.id = l.link_id AND t.issue_id IN (SELECT id FROM subtree)
        ))
        OR (l.link_type = 'project' AND l.link_id = ANY(sqlc.arg('project_ids')::uuid[]))
    )
)
SELECT count(*)::bigint AS count,
       count(*) FILTER (WHERE q.blocks_work)::bigint AS blocking_count
FROM question q
WHERE q.workspace_id = sqlc.arg('workspace_id')::uuid
  AND q.status IN ('open', 'partly_answered')
  AND q.id IN (SELECT question_id FROM below);
