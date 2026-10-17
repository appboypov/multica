-- Seed current obligations before retiring their notices. The migration runner
-- sends this file as one implicit transaction, so an insert failure leaves the
-- notices untouched. Creator migration 753 runs first.
-- A question's issue is the first issue its links reach, through an issue,
-- comment or task link, as the live listener reads it; one that reaches none
-- carries its ask's title or prompt. Its actor is the member or agent who asked.
INSERT INTO todo (workspace_id, member_id, kind, source_key, issue_id,
                  question_id, actor_type, actor_id, title, body, created_at)
SELECT q.workspace_id, qa.user_id, 'question', q.id::text, i.id,
       q.id, CASE WHEN q.asker_user_id IS NOT NULL THEN 'member' ELSE 'agent' END,
       COALESCE(q.asker_user_id, q.asker_agent_id), COALESCE(i.title, q.ask_title, q.prompt), q.prompt, q.created_at
FROM question q
JOIN question_answerer qa ON qa.question_id = q.id AND qa.workspace_id = q.workspace_id
JOIN member m ON m.workspace_id = q.workspace_id AND m.user_id = qa.user_id
LEFT JOIN LATERAL (
    SELECT i.id, i.title
    FROM question_link l
    LEFT JOIN comment c ON l.link_type = 'comment' AND c.id = l.link_id
        AND c.workspace_id = l.workspace_id AND c.deleted_at IS NULL
    LEFT JOIN agent_task_queue t ON l.link_type = 'task' AND t.id = l.link_id
        AND EXISTS (SELECT 1 FROM agent a WHERE a.id = t.agent_id AND a.workspace_id = l.workspace_id)
    JOIN issue i ON i.workspace_id = l.workspace_id AND i.id = CASE l.link_type
        WHEN 'issue' THEN l.link_id WHEN 'comment' THEN c.issue_id WHEN 'task' THEN t.issue_id END
    WHERE l.question_id = q.id AND l.workspace_id = q.workspace_id
    ORDER BY l.position, l.link_id
    LIMIT 1
) i ON true
WHERE q.status IN ('open', 'partly_answered')
  AND NOT EXISTS (
      SELECT 1 FROM question_answer a
      WHERE a.question_id = q.id AND a.workspace_id = q.workspace_id
        AND a.answerer_user_id = qa.user_id
  )
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

INSERT INTO todo (workspace_id, member_id, kind, source_key, issue_id, title, created_at)
SELECT i.workspace_id, i.assignee_id, 'assigned', i.id::text, i.id, i.title, i.updated_at
FROM issue i
JOIN member m ON m.workspace_id = i.workspace_id AND m.user_id = i.assignee_id
WHERE i.assignee_type = 'member' AND i.status IN ('todo', 'blocked', 'in_review')
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

INSERT INTO todo (workspace_id, member_id, kind, source_key, issue_id, title, created_at)
SELECT i.workspace_id, i.creator_id, 'review', i.id::text, i.id, i.title, i.updated_at
FROM issue i
JOIN member m ON m.workspace_id = i.workspace_id AND m.user_id = i.creator_id
WHERE i.creator_type = 'member' AND i.status = 'in_review'
  AND NOT COALESCE(i.assignee_type = 'member' AND i.assignee_id = i.creator_id, false)
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

-- Read the latest run that failed or was dispatched, and attribute it to its
-- originator, accountable member, source run's originator, then source run's
-- accountable member. A newer run closes the failure only once dispatched, and
-- a later status change closes it too, as both do live.
INSERT INTO todo (workspace_id, member_id, kind, source_key, issue_id, task_id,
                  actor_type, actor_id, title, created_at)
SELECT i.workspace_id, m.user_id, 'run_failed', t.id::text, i.id, t.id,
       'agent', t.agent_id, i.title, COALESCE(t.completed_at, t.created_at)
FROM issue i
JOIN LATERAL (
    SELECT t.* FROM agent_task_queue t
    JOIN agent a ON a.id = t.agent_id AND a.workspace_id = i.workspace_id
    WHERE t.issue_id = i.id
      AND (t.status = 'failed' OR COALESCE(t.dispatched_at, t.started_at) IS NOT NULL)
    ORDER BY t.created_at DESC, t.id DESC LIMIT 1
) t ON t.status = 'failed'
LEFT JOIN agent_task_queue source ON source.id = t.delegated_from_task_id
    AND EXISTS (SELECT 1 FROM agent a WHERE a.id = source.agent_id AND a.workspace_id = i.workspace_id)
JOIN member m ON m.workspace_id = i.workspace_id
    AND m.user_id = COALESCE(t.originator_user_id, t.accountable_user_id, source.originator_user_id, source.accountable_user_id)
WHERE NOT EXISTS (
    SELECT 1 FROM activity_log s
    WHERE s.workspace_id = i.workspace_id AND s.issue_id = i.id AND s.action = 'status_changed'
      AND s.created_at > COALESCE(t.completed_at, t.created_at)
)
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

INSERT INTO todo (workspace_id, member_id, kind, source_key, issue_id, task_id,
                  actor_type, actor_id, title, created_at)
SELECT i.workspace_id, m.user_id, 'agent_blocked', i.id::text, i.id, t.id,
       'agent', t.agent_id, i.title, s.created_at
FROM issue i
JOIN LATERAL (
    SELECT t.* FROM agent_task_queue t
    JOIN agent a ON a.id = t.agent_id AND a.workspace_id = i.workspace_id
    WHERE t.issue_id = i.id AND COALESCE(t.dispatched_at, t.started_at) IS NOT NULL
    ORDER BY COALESCE(t.dispatched_at, t.started_at) DESC, t.id DESC LIMIT 1
) t ON true
JOIN LATERAL (
    SELECT s.* FROM activity_log s
    WHERE s.workspace_id = i.workspace_id AND s.issue_id = i.id AND s.action = 'status_changed'
    ORDER BY s.created_at DESC, s.id DESC LIMIT 1
) s ON s.actor_type = 'agent' AND s.actor_id = t.agent_id AND s.details->>'to' = 'blocked'
LEFT JOIN agent_task_queue source ON source.id = t.delegated_from_task_id
    AND EXISTS (SELECT 1 FROM agent a WHERE a.id = source.agent_id AND a.workspace_id = i.workspace_id)
JOIN member m ON m.workspace_id = i.workspace_id
    AND m.user_id = COALESCE(t.originator_user_id, t.accountable_user_id, source.originator_user_id, source.accountable_user_id)
WHERE i.status = 'blocked' AND i.assignee_type IN ('agent', 'squad')
  AND COALESCE(t.dispatched_at, t.started_at) <= s.created_at
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

-- Only unarchived mention notices are eligible, not every historical mention.
-- Description mentions have no comment pointer and use the issue as their key.
INSERT INTO todo (workspace_id, member_id, kind, source_key, issue_id, comment_id,
                  actor_type, actor_id, title, created_at)
SELECT n.workspace_id, n.recipient_id, 'mention', COALESCE(c.id, i.id)::text, i.id, c.id,
       CASE WHEN n.actor_type IN ('member', 'agent') THEN n.actor_type END,
       n.actor_id, i.title, n.created_at
FROM inbox_item n
JOIN issue i ON i.id = n.issue_id AND i.workspace_id = n.workspace_id
JOIN member m ON m.workspace_id = n.workspace_id AND m.user_id = n.recipient_id
LEFT JOIN comment c ON c.id::text = n.details->>'comment_id' AND c.issue_id = i.id
WHERE n.type = 'mentioned' AND n.recipient_type = 'member' AND NOT n.archived
  AND (NULLIF(n.details->>'comment_id', '') IS NULL OR c.id IS NOT NULL)
  AND NOT COALESCE(n.actor_type = 'member' AND n.actor_id = n.recipient_id, false)
  AND NOT EXISTS (
      SELECT 1 FROM comment reply
      WHERE reply.issue_id = i.id AND reply.author_type = 'member'
        AND reply.author_id = n.recipient_id AND reply.created_at > n.created_at
  )
ORDER BY n.created_at DESC, n.id DESC
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

INSERT INTO todo (workspace_id, member_id, kind, source_key, autopilot_id, title, body, details, created_at)
SELECT n.workspace_id, n.recipient_id, 'autopilot_paused', a.id::text, a.id,
       a.title, n.body, n.details, n.created_at
FROM inbox_item n
JOIN autopilot a ON a.id::text = n.details->>'autopilot_id' AND a.workspace_id = n.workspace_id
JOIN member m ON m.workspace_id = n.workspace_id AND m.user_id = n.recipient_id
WHERE n.type = 'autopilot_paused' AND NOT n.archived AND n.recipient_type = 'member' AND a.status = 'paused'
ORDER BY n.created_at DESC, n.id DESC
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

INSERT INTO todo (workspace_id, member_id, kind, source_key, title, body, details, created_at)
SELECT n.workspace_id, n.recipient_id, 'quota_reached',
       to_char(p.period_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') || '/' ||
       to_char(p.period_end AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
       n.title, n.body, n.details, n.created_at
FROM inbox_item n
JOIN autopilot_quota_period p ON p.workspace_id = n.workspace_id
    AND n.created_at >= p.period_start AND n.created_at < p.period_end
JOIN member m ON m.workspace_id = n.workspace_id AND m.user_id = n.recipient_id
WHERE n.type = 'autopilot_quota_exceeded' AND NOT n.archived AND n.recipient_type = 'member'
  AND NOT EXISTS (
      SELECT 1 FROM autopilot_quota_reservation r
      WHERE r.workspace_id = n.workspace_id AND r.created_at > n.created_at
  )
ORDER BY n.created_at DESC, n.id DESC
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

-- A successful retry/rerun handles every ancestor, not just its direct parent.
WITH RECURSIVE successful_quick_creates AS (
    SELECT i.workspace_id, t.id, COALESCE(t.retry_of_task_id, t.rerun_of_task_id) AS parent_id
    FROM issue i
    JOIN agent_task_queue t ON t.id = i.origin_id
    WHERE i.origin_type = 'quick_create'
    UNION
    SELECT s.workspace_id, t.id, COALESCE(t.retry_of_task_id, t.rerun_of_task_id)
    FROM successful_quick_creates s
    JOIN agent_task_queue t ON t.id = s.parent_id
    JOIN agent a ON a.id = t.agent_id AND a.workspace_id = s.workspace_id
)
INSERT INTO todo (workspace_id, member_id, kind, source_key, task_id, title, body, details, created_at)
SELECT n.workspace_id, n.recipient_id, 'quick_create_failed', t.id::text, t.id,
       COALESCE(n.details->>'original_prompt', t.context->>'prompt', n.title), n.body,
       n.details || jsonb_build_object('reason', CASE WHEN n.type = 'quick_create_unconfirmed' THEN 'unconfirmed' ELSE 'failed' END),
       n.created_at
FROM inbox_item n
JOIN agent_task_queue t ON t.id::text = n.details->>'task_id'
JOIN agent a ON a.id = t.agent_id AND a.workspace_id = n.workspace_id
JOIN member m ON m.workspace_id = n.workspace_id AND m.user_id = n.recipient_id
WHERE n.type IN ('quick_create_failed', 'quick_create_unconfirmed') AND NOT n.archived AND n.recipient_type = 'member'
  AND NOT EXISTS (
      SELECT 1 FROM successful_quick_creates s WHERE s.workspace_id = n.workspace_id AND s.id = t.id
  )
ORDER BY n.created_at DESC, n.id DESC
ON CONFLICT (workspace_id, member_id, kind, source_key) WHERE done_at IS NULL DO NOTHING;

-- Resolved notices are archived too: they no longer belong in Updates and
-- must not resurrect an obligation already handled in the source data.
UPDATE inbox_item SET archived = true
WHERE NOT archived AND type IN (
    'question_asked', 'question_reassigned', 'question_overdue', 'mentioned',
    'task_failed', 'autopilot_paused', 'autopilot_quota_exceeded',
    'quick_create_failed', 'quick_create_unconfirmed'
);
