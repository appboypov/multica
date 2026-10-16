CREATE TABLE question_wake_pending (
    workspace_id uuid NOT NULL,
    issue_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    actor_user_id uuid NOT NULL,
    source_question_id uuid NOT NULL,
    answer_anchor timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (workspace_id, issue_id, agent_id)
);

-- One selection boundary for cards, waiting drains and claim hydration. A
-- retained wait advances only when a question-wake run actually starts; an
-- ordinary retry has not received that wait's question input.
CREATE FUNCTION question_wake_answer_cutoff(
    p_workspace_id uuid, p_issue_id uuid, p_agent_id uuid,
    p_exclude_task_id uuid, p_answer_anchor timestamptz
) RETURNS timestamptz
LANGUAGE sql STABLE AS $$
    WITH RECURSIVE excluded_tasks AS (
        SELECT id, retry_of_task_id FROM agent_task_queue
        WHERE id = p_exclude_task_id
          AND agent_id = p_agent_id AND issue_id = p_issue_id
        UNION
        SELECT t.id, t.retry_of_task_id FROM agent_task_queue t
        JOIN excluded_tasks child ON t.id = child.retry_of_task_id
        WHERE t.agent_id = p_agent_id AND t.issue_id = p_issue_id
    ), anchor AS (
        SELECT COALESCE(p_answer_anchor, (
            SELECT pending.answer_anchor FROM question_wake_pending pending
            WHERE pending.workspace_id = p_workspace_id
              AND pending.issue_id = p_issue_id AND pending.agent_id = p_agent_id
        )) AS value
    )
    SELECT GREATEST(anchor.value, COALESCE((
        SELECT max(t.dispatched_at) FROM agent_task_queue t
        JOIN agent a ON a.id = t.agent_id AND a.workspace_id = p_workspace_id
        JOIN issue i ON i.id = t.issue_id AND i.workspace_id = p_workspace_id
        WHERE t.agent_id = p_agent_id AND t.issue_id = p_issue_id
          AND t.dispatched_at IS NOT NULL AND t.started_at IS NOT NULL
          AND t.id NOT IN (SELECT id FROM excluded_tasks)
          AND (anchor.value IS NULL OR t.trigger_evidence_kind = 'question_wake')
    ), '-infinity'::timestamptz))
    FROM anchor;
$$;
