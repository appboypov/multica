-- Questions agents ask in a run or members ask on an issue (ADR-0021). Every query is
-- scoped by workspace_id; the service validates ids and runs the multi-row
-- writes of one action in one transaction.

-- name: CreateQuestion :one
-- One question of an ask: a one-question ask passes ask_id = id and
-- ask_position 0; ask_title is set only when the ask holds several.
INSERT INTO question (
    id, workspace_id, asker_agent_id, asked_in_task_id, asker_user_id, prompt, context_line,
    answer_type, settings, suggested_answer, due_date, blocks_work,
    ask_id, ask_position, ask_title, answer_policy
) VALUES (
    @id, @workspace_id, sqlc.narg('asker_agent_id'), sqlc.narg('asked_in_task_id'), sqlc.narg('asker_user_id'), @prompt, sqlc.narg('context_line'),
    @answer_type, @settings, sqlc.narg('suggested_answer'), sqlc.narg('due_date'), @blocks_work,
    @ask_id, @ask_position, sqlc.narg('ask_title'), @answer_policy
)
RETURNING *;

-- name: CreateQuestionLink :exec
-- Position 0 is where the question was asked.
INSERT INTO question_link (question_id, workspace_id, link_type, link_id, position)
VALUES (@question_id, @workspace_id, @link_type, @link_id, @position);

-- name: AppendQuestionLinks :one
-- Caller holds the question lock, serializing positions with other appends.
INSERT INTO question_link (question_id, workspace_id, link_type, link_id, position)
SELECT q.id, q.workspace_id, @link_type, @link_id,
       COALESCE((SELECT max(l.position) FROM question_link l
                 WHERE l.question_id = q.id AND l.workspace_id = q.workspace_id), -1) + 1
FROM question q
WHERE q.id = @question_id AND q.workspace_id = @workspace_id
ON CONFLICT (question_id, link_type, link_id) DO NOTHING
RETURNING *;

-- name: CreateQuestionAnswerers :exec
INSERT INTO question_answerer (question_id, workspace_id, user_id)
SELECT @question_id, @workspace_id, answerer.user_id
FROM unnest(@user_ids::uuid[]) AS answerer(user_id)
ON CONFLICT (question_id, user_id) DO NOTHING;

-- name: CreateQuestionEvent :one
INSERT INTO question_event (id, question_id, workspace_id, kind, actor_type, actor_id, details)
VALUES (@id, @question_id, @workspace_id, @kind, @actor_type, sqlc.narg('actor_id'), @details)
RETURNING *;

-- name: GetQuestion :one
SELECT * FROM question
WHERE id = @id AND workspace_id = @workspace_id;

-- name: GetQuestionForUpdate :one
-- Serializes the actions on one question: answering, reassigning, closing and
-- withdrawing each check the status and the answerers under this lock.
SELECT * FROM question
WHERE id = @id AND workspace_id = @workspace_id
FOR UPDATE;

-- name: ListQuestionsByIssue :many
-- Direct cards include links to the issue, its live comments and its runs.
SELECT DISTINCT q.* FROM question q
JOIN question_link l
  ON l.question_id = q.id AND l.workspace_id = q.workspace_id
WHERE q.workspace_id = @workspace_id
  AND (
      (l.link_type = 'issue' AND l.link_id = @issue_id)
      OR (l.link_type = 'comment' AND EXISTS (
          SELECT 1 FROM comment c
          WHERE c.id = l.link_id AND c.workspace_id = @workspace_id
            AND c.issue_id = @issue_id AND c.deleted_at IS NULL
      ))
      OR (l.link_type = 'task' AND EXISTS (
          SELECT 1 FROM agent_task_queue t
          JOIN agent a ON a.id = t.agent_id AND a.workspace_id = @workspace_id
          WHERE t.id = l.link_id AND t.issue_id = @issue_id
      ))
  )
ORDER BY q.created_at ASC, q.id ASC;

-- name: ListQuestionsByAsk :many
-- An ask's questions in order (idx_question_ask).
SELECT * FROM question
WHERE workspace_id = @workspace_id
  AND ask_id = @ask_id
ORDER BY ask_position ASC;

-- name: ListQuestionLinksByQuestions :many
SELECT * FROM question_link
WHERE workspace_id = @workspace_id
  AND question_id = ANY(@question_ids::uuid[])
ORDER BY question_id, position ASC;

-- name: ListQuestionAnswerersByQuestions :many
SELECT * FROM question_answerer
WHERE workspace_id = @workspace_id
  AND question_id = ANY(@question_ids::uuid[])
ORDER BY question_id, assigned_at ASC, user_id ASC;

-- name: ListQuestionAnswersByQuestions :many
SELECT * FROM question_answer
WHERE workspace_id = @workspace_id
  AND question_id = ANY(@question_ids::uuid[])
ORDER BY question_id, created_at ASC, id ASC;

-- name: ListCurrentAnswers :many
-- The answers of a question's current answerers, oldest first: the answer of
-- someone reassigned away stays in the history but does not count toward the
-- answer policy (design.md decision 3).
SELECT a.* FROM question_answer a
JOIN question_answerer r
  ON r.question_id = a.question_id
 AND r.workspace_id = a.workspace_id
 AND r.user_id = a.answerer_user_id
WHERE a.workspace_id = @workspace_id
  AND a.question_id = @question_id
ORDER BY a.created_at ASC, a.id ASC;

-- name: ListQuestionEventsByQuestions :many
SELECT * FROM question_event
WHERE workspace_id = @workspace_id
  AND question_id = ANY(@question_ids::uuid[])
ORDER BY question_id, created_at ASC, id ASC;

-- name: IsQuestionAnswerer :one
SELECT EXISTS (
    SELECT 1 FROM question_answerer
    WHERE question_id = @question_id
      AND workspace_id = @workspace_id
      AND user_id = @user_id
) AS is_answerer;

-- name: UpsertQuestionAnswer :one
-- One answer per answerer, edited in place (idx_question_answer_answerer).
-- inserted tells a first answer from an edit, which write different events.
INSERT INTO question_answer (id, question_id, workspace_id, answerer_user_id, value, note)
VALUES (@id, @question_id, @workspace_id, @answerer_user_id, @value, sqlc.narg('note'))
ON CONFLICT (question_id, answerer_user_id) DO UPDATE SET
    value = EXCLUDED.value,
    note = EXCLUDED.note,
    updated_at = clock_timestamp()
WHERE question_answer.workspace_id = EXCLUDED.workspace_id
RETURNING *, (xmax = 0) AS inserted;

-- name: SetQuestionStatus :one
-- Moves a question to a status only from one of the allowed ones, so a
-- transition that lost a race returns no row instead of overwriting.
UPDATE question
SET status = @status,
    updated_at = now()
WHERE id = @id
  AND workspace_id = @workspace_id
  AND status = ANY(@from_statuses::text[])
RETURNING *;

-- name: ReplaceQuestionAnswerer :one
-- Reassign: the new answerer takes the old one's place. removed_count is 0
-- when the old user was not an answerer, and nothing changes then.
WITH removed AS (
    DELETE FROM question_answerer
    WHERE question_answerer.question_id = @question_id
      AND question_answerer.workspace_id = @workspace_id
      AND question_answerer.user_id = @from_user_id
    RETURNING question_answerer.question_id
), added AS (
    INSERT INTO question_answerer (question_id, workspace_id, user_id)
    SELECT removed.question_id, @workspace_id, @to_user_id
    FROM removed
    ON CONFLICT (question_id, user_id) DO NOTHING
    RETURNING question_answerer.user_id
)
SELECT count(*)::bigint AS removed_count FROM removed;

-- name: ListOpenOverdueQuestions :many
-- The overdue sweep runs across workspaces: open and partly answered questions
-- due before the given UTC day that have had no overdue notice yet, each with
-- its ask_id and the issue it is linked to.
SELECT q.*, l.link_id AS issue_id
FROM question q
JOIN question_link l
  ON l.question_id = q.id
 AND l.workspace_id = q.workspace_id
 AND l.link_type = 'issue'
WHERE q.status IN ('open', 'partly_answered')
  AND q.overdue_notified_at IS NULL
  AND q.due_date < @before_date::date
ORDER BY q.due_date ASC, q.id ASC
LIMIT @max_rows;

-- name: StampQuestionOverdueNotified :execrows
-- 1 when this call stamped the question; 0 when another tick already did, or
-- it is no longer open, so the overdue notice goes out once.
UPDATE question
SET overdue_notified_at = now()
WHERE id = @id
  AND workspace_id = @workspace_id
  AND status IN ('open', 'partly_answered')
  AND overdue_notified_at IS NULL;

-- The caller resolves the target's live wake scope. Each selector uses the same
-- effective cutoff, with an optional retained anchor and claiming retry exclusion.

-- name: CountUnseenQuestionAnswers :one

SELECT count(*)::bigint AS unseen_count
FROM question_answer a
JOIN question q ON q.id = a.question_id AND q.workspace_id = a.workspace_id
WHERE a.workspace_id = @workspace_id
  AND q.id = ANY(@question_ids::uuid[])
  AND q.status IN ('open', 'partly_answered', 'answered', 'cancelled')
  AND a.updated_at > (SELECT question_wake_answer_cutoff(
      @workspace_id::uuid, sqlc.narg('issue_id')::uuid, sqlc.narg('chat_session_id')::uuid, @agent_id::uuid,
      sqlc.narg('exclude_task_id')::uuid, sqlc.narg('answer_anchor')::timestamptz))
  AND NOT EXISTS (
      SELECT 1 FROM task_supplement s
      JOIN agent_task_queue handled_run ON handled_run.id = s.task_id
      WHERE s.workspace_id = a.workspace_id
        AND s.item_kind = 'answer'
        AND s.handled_at IS NOT NULL
        AND handled_run.agent_id = @agent_id
        AND s.item_id = (
            SELECT e.id FROM question_event e
            WHERE e.workspace_id = a.workspace_id
              AND e.question_id = a.question_id
              AND e.actor_type = 'member'
              AND e.actor_id = a.answerer_user_id
              AND e.kind IN ('answered', 'answer_edited')
            ORDER BY e.created_at DESC, e.id DESC
            LIMIT 1
        )
  );

-- name: ListUnseenQuestionAnswers :many

SELECT a.id, a.question_id, a.answerer_user_id, a.value, a.note, a.created_at, a.updated_at,
       q.prompt, q.answer_type, q.settings, q.asker_agent_id,
       asker.name AS asker_name, u.name AS answerer_name
FROM question_answer a
JOIN question q ON q.id = a.question_id AND q.workspace_id = a.workspace_id
JOIN agent asker ON asker.id = q.asker_agent_id AND asker.workspace_id = q.workspace_id
JOIN "user" u ON u.id = a.answerer_user_id
WHERE a.workspace_id = @workspace_id
  AND q.id = ANY(@question_ids::uuid[])
  AND q.status IN ('open', 'partly_answered', 'answered', 'cancelled')
  AND a.updated_at > (SELECT question_wake_answer_cutoff(
      @workspace_id::uuid, sqlc.narg('issue_id')::uuid, sqlc.narg('chat_session_id')::uuid, @agent_id::uuid,
      sqlc.narg('exclude_task_id')::uuid, sqlc.narg('answer_anchor')::timestamptz))
  AND NOT EXISTS (
      SELECT 1 FROM task_supplement s
      JOIN agent_task_queue handled_run ON handled_run.id = s.task_id
      WHERE s.workspace_id = a.workspace_id
        AND s.item_kind = 'answer'
        AND s.handled_at IS NOT NULL
        AND handled_run.agent_id = @agent_id
        AND s.item_id = (
            SELECT e.id FROM question_event e
            WHERE e.workspace_id = a.workspace_id
              AND e.question_id = a.question_id
              AND e.actor_type = 'member'
              AND e.actor_id = a.answerer_user_id
              AND e.kind IN ('answered', 'answer_edited')
            ORDER BY e.created_at DESC, e.id DESC
            LIMIT 1
        )
  )
ORDER BY a.updated_at ASC, a.id ASC;

-- name: ListUnseenQuestionAskBacks :many
-- Ask-backs are no answers: they count toward nothing and ride along in the
-- next woken run, against the same anchor.
SELECT e.id, e.question_id, e.actor_id, e.details, e.created_at,
       q.prompt, q.asker_agent_id, asker.name AS asker_name, u.name AS actor_name
FROM question_event e
JOIN question q ON q.id = e.question_id AND q.workspace_id = e.workspace_id
JOIN agent asker ON asker.id = q.asker_agent_id AND asker.workspace_id = q.workspace_id
JOIN "user" u ON u.id = e.actor_id
WHERE e.workspace_id = @workspace_id
  AND e.kind = 'ask_back'
  AND q.id = ANY(@question_ids::uuid[])
  AND q.status IN ('open', 'partly_answered', 'answered')
  AND e.created_at > (SELECT question_wake_answer_cutoff(
      @workspace_id::uuid, @issue_id::uuid, NULL::uuid, @agent_id::uuid,
      sqlc.narg('exclude_task_id')::uuid, sqlc.narg('answer_anchor')::timestamptz))
  AND NOT EXISTS (
      SELECT 1 FROM task_supplement s
      JOIN agent_task_queue handled_run ON handled_run.id = s.task_id
      WHERE s.workspace_id = e.workspace_id AND s.item_kind = 'answer'
        AND s.item_id = e.id AND s.handled_at IS NOT NULL
        AND handled_run.agent_id = @agent_id
  )
ORDER BY e.created_at ASC, e.id ASC;

-- name: DeleteQuestionLinksTo :many
DELETE FROM question_link
WHERE workspace_id = @workspace_id AND link_type = @link_type
  AND link_id = ANY(@ids::uuid[])
RETURNING question_id;

-- name: DeleteQuestionLinksOfIssues :many
DELETE FROM question_link l
WHERE l.workspace_id = @workspace_id
  AND (
      (l.link_type = 'issue' AND l.link_id = ANY(@issue_ids::uuid[]))
      OR (l.link_type = 'comment' AND EXISTS (
          SELECT 1 FROM comment c WHERE c.id = l.link_id
            AND c.workspace_id = @workspace_id AND c.issue_id = ANY(@issue_ids::uuid[])
      ))
      OR (l.link_type = 'task' AND EXISTS (
          SELECT 1 FROM agent_task_queue t
          JOIN agent a ON a.id = t.agent_id AND a.workspace_id = @workspace_id
          WHERE t.id = l.link_id AND t.issue_id = ANY(@issue_ids::uuid[])
      ))
  )
RETURNING l.question_id;

-- name: WithdrawQuestionsWithoutLinks :many
UPDATE question q SET status = 'withdrawn', updated_at = now()
WHERE q.workspace_id = @workspace_id AND q.id = ANY(@question_ids::uuid[])
  AND q.status IN ('open', 'partly_answered', 'answered')
  AND NOT EXISTS (
      SELECT 1 FROM question_link l
      WHERE l.question_id = q.id AND l.workspace_id = q.workspace_id
  )
RETURNING q.id;
