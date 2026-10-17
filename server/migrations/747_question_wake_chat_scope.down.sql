-- Reverse 747: restore 745's issue-only cutoff.
DROP FUNCTION question_wake_answer_cutoff(uuid, uuid, uuid, uuid, uuid, timestamptz);

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
