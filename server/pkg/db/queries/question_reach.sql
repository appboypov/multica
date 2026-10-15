-- Live read-time reach (ADR-0007). UNION, rather than UNION ALL, in every
-- recursive tree walk makes malformed parent cycles finite.

-- name: ListQuestionReach :many
WITH RECURSIVE links AS (
    SELECT l.*,
           CASE WHEN l.link_type = 'issue' THEN i.id
                WHEN l.link_type = 'comment' THEN c.issue_id
                WHEN l.link_type = 'task' THEN t.issue_id END::uuid AS issue_id
    FROM question_link l
    JOIN question q ON q.id = l.question_id AND q.workspace_id = l.workspace_id
    LEFT JOIN issue i ON l.link_type = 'issue' AND i.id = l.link_id AND i.workspace_id = @workspace_id
    LEFT JOIN comment c ON l.link_type = 'comment' AND c.id = l.link_id
      AND c.workspace_id = @workspace_id AND c.deleted_at IS NULL
    LEFT JOIN agent_task_queue t ON l.link_type = 'task' AND t.id = l.link_id
      AND EXISTS (SELECT 1 FROM agent a WHERE a.id = t.agent_id AND a.workspace_id = @workspace_id)
    WHERE l.workspace_id = @workspace_id AND q.id = ANY(@question_ids::uuid[])
), objects AS (
    SELECT 'issue'::text AS object_type, i.id AS object_id, i.title,
           i.status, i.assignee_type, i.assignee_id
    FROM issue i WHERE i.workspace_id = @workspace_id
    UNION ALL
    SELECT 'comment', c.id, ''::text, ''::text, NULL::text, NULL::uuid
    FROM comment c WHERE c.workspace_id = @workspace_id AND c.deleted_at IS NULL
    UNION ALL
    SELECT 'task', t.id, ''::text, t.status, NULL::text, NULL::uuid
    FROM agent_task_queue t JOIN agent a ON a.id = t.agent_id AND a.workspace_id = @workspace_id
    JOIN issue i ON i.id = t.issue_id AND i.workspace_id = @workspace_id
    UNION ALL
    SELECT 'project', p.id, p.title, p.status, p.lead_type, p.lead_id
    FROM project p WHERE p.workspace_id = @workspace_id
    UNION ALL
    SELECT 'autopilot', a.id, a.title, a.status, a.assignee_type, a.assignee_id
    FROM autopilot a WHERE a.workspace_id = @workspace_id
    UNION ALL
    SELECT 'agent', a.id, a.name, a.status, NULL::text, NULL::uuid
    FROM agent a WHERE a.workspace_id = @workspace_id
    UNION ALL
    SELECT 'squad', s.id, s.name, ''::text, NULL::text, NULL::uuid
    FROM squad s WHERE s.workspace_id = @workspace_id
    UNION ALL
    SELECT 'skill', s.id, s.name, ''::text, NULL::text, NULL::uuid
    FROM skill s WHERE s.workspace_id = @workspace_id
), ancestors AS (
    SELECT l.question_id, l.link_type, l.link_id, i.id AS issue_id
    FROM links l JOIN issue i ON i.id = l.issue_id AND i.workspace_id = @workspace_id
    UNION
    SELECT a.question_id, a.link_type, a.link_id, parent.id
    FROM ancestors a
    JOIN issue child ON child.id = a.issue_id AND child.workspace_id = @workspace_id
    JOIN issue parent ON parent.id = child.parent_issue_id AND parent.workspace_id = @workspace_id
), reached AS (
    SELECT l.question_id, l.link_type, l.link_id, o.object_type, o.object_id
    FROM links l JOIN objects o ON o.object_type = l.link_type AND o.object_id = l.link_id
    UNION
    SELECT a.question_id, a.link_type, a.link_id, 'issue', a.issue_id FROM ancestors a
    UNION
    SELECT a.question_id, a.link_type, a.link_id, 'project', p.id
    FROM ancestors a
    JOIN issue i ON i.id = a.issue_id AND i.workspace_id = @workspace_id
    JOIN project p ON p.id = i.project_id AND p.workspace_id = @workspace_id
)
SELECT r.question_id, r.link_type, r.link_id, l.position,
       r.object_type::text AS object_type, r.object_id,
       o.title::text AS object_title, o.status::text AS object_status,
       COALESCE(o.assignee_type, '')::text AS assignee_type, o.assignee_id,
       l.issue_id::uuid AS issue_id, source.parent_issue_id,
       CASE WHEN l.link_type = 'issue' THEN COALESCE(w.issue_prefix || '-' || source.number::text, '') ELSE '' END::text AS identifier,
       COALESCE(w.issue_prefix || '-' || source.number::text, '')::text AS issue_identifier
FROM reached r
JOIN links l ON l.question_id = r.question_id AND l.link_type = r.link_type AND l.link_id = r.link_id
JOIN objects o ON o.object_type = r.object_type AND o.object_id = r.object_id
LEFT JOIN issue source ON source.id = l.issue_id AND source.workspace_id = @workspace_id
LEFT JOIN workspace w ON w.id = source.workspace_id
ORDER BY r.question_id, l.position, r.link_type, r.link_id, r.object_type, r.object_id;

-- name: SubtreeIssueIDs :many
WITH RECURSIVE subtree AS (
    SELECT i.id FROM issue i
    WHERE i.workspace_id = @workspace_id AND i.id = ANY(@issue_ids::uuid[])
    UNION
    SELECT child.id FROM issue child JOIN subtree parent ON child.parent_issue_id = parent.id
    WHERE child.workspace_id = @workspace_id
)
SELECT id FROM subtree ORDER BY id;

-- name: QuestionIDsBelow :many
WITH RECURSIVE subtree AS (
    SELECT i.id FROM issue i WHERE i.workspace_id = @workspace_id
      AND (i.id = ANY(@issue_ids::uuid[]) OR i.project_id = ANY(@project_ids::uuid[]))
    UNION
    SELECT child.id FROM issue child JOIN subtree parent ON child.parent_issue_id = parent.id
    WHERE child.workspace_id = @workspace_id
)
SELECT DISTINCT l.question_id FROM question_link l
JOIN question q ON q.id = l.question_id AND q.workspace_id = l.workspace_id
WHERE l.workspace_id = @workspace_id AND (
    (l.link_type = 'issue' AND l.link_id IN (SELECT id FROM subtree))
    OR (l.link_type = 'comment' AND EXISTS (
        SELECT 1 FROM comment c WHERE c.id = l.link_id AND c.workspace_id = @workspace_id
          AND c.deleted_at IS NULL AND c.issue_id IN (SELECT id FROM subtree)
    ))
    OR (l.link_type = 'task' AND EXISTS (
        SELECT 1 FROM agent_task_queue t JOIN agent a ON a.id = t.agent_id AND a.workspace_id = @workspace_id
        WHERE t.id = l.link_id AND t.issue_id IN (SELECT id FROM subtree)
    ))
    OR (l.link_type = 'project' AND EXISTS (
        SELECT 1 FROM project p WHERE p.id = l.link_id AND p.workspace_id = @workspace_id
          AND p.id = ANY(@project_ids::uuid[])
    ))
)
ORDER BY l.question_id;

-- name: WorkSeedsForMember :many
SELECT 'issue'::text AS object_type, i.id AS object_id
FROM issue i WHERE i.workspace_id = @workspace_id
  AND i.assignee_type = 'member' AND i.assignee_id = @user_id
UNION
SELECT 'issue', i.id FROM issue i
JOIN project p ON p.id = i.project_id AND p.workspace_id = @workspace_id
WHERE i.workspace_id = @workspace_id AND p.lead_type = 'member' AND p.lead_id = @user_id
UNION
SELECT 'project', p.id FROM project p
WHERE p.workspace_id = @workspace_id AND p.lead_type = 'member' AND p.lead_id = @user_id;

-- name: ListQuestionWakeScope :many
-- Issue assignment reaches every descendant; project leadership reaches every
-- descendant of its project issues, but wakes on each question's asking issue.
WITH RECURSIVE owned_tree AS (
    SELECT i.id, 'issue'::text AS source_type FROM issue i
    WHERE i.workspace_id = @workspace_id AND i.id = @issue_id
      AND i.assignee_type = 'agent' AND i.assignee_id = @agent_id
    UNION
    SELECT i.id, 'project'::text FROM issue i
    JOIN project p ON p.id = i.project_id AND p.workspace_id = @workspace_id
    WHERE i.workspace_id = @workspace_id AND p.lead_type = 'agent' AND p.lead_id = @agent_id
    UNION
    SELECT child.id, parent.source_type FROM issue child
    JOIN owned_tree parent ON child.parent_issue_id = parent.id
    WHERE child.workspace_id = @workspace_id
), linked_issues AS (
    SELECT l.question_id, l.link_id AS issue_id FROM question_link l
    JOIN issue i ON i.id = l.link_id AND i.workspace_id = @workspace_id
    WHERE l.workspace_id = @workspace_id AND l.link_type = 'issue'
    UNION
    SELECT l.question_id, c.issue_id FROM question_link l
    JOIN comment c ON c.id = l.link_id AND c.workspace_id = @workspace_id AND c.deleted_at IS NULL
    WHERE l.workspace_id = @workspace_id AND l.link_type = 'comment'
    UNION
    SELECT l.question_id, t.issue_id FROM question_link l
    JOIN agent_task_queue t ON t.id = l.link_id
    JOIN agent a ON a.id = t.agent_id AND a.workspace_id = @workspace_id
    WHERE l.workspace_id = @workspace_id AND l.link_type = 'task'
)
SELECT DISTINCT q.id FROM question q
JOIN agent_task_queue asking ON asking.id = q.asked_in_task_id
JOIN agent asker ON asker.id = asking.agent_id AND asker.workspace_id = @workspace_id
LEFT JOIN issue asking_issue ON asking_issue.id = asking.issue_id AND asking_issue.workspace_id = @workspace_id
JOIN agent target ON target.id = @agent_id AND target.workspace_id = @workspace_id
JOIN issue target_issue ON target_issue.id = @issue_id AND target_issue.workspace_id = @workspace_id
WHERE q.workspace_id = @workspace_id AND q.status IN ('open', 'partly_answered', 'answered', 'cancelled')
  AND (asking.issue_id IS NULL OR asking_issue.id IS NOT NULL)
  AND (
    (q.asker_agent_id = @agent_id AND asking.issue_id = @issue_id)
    OR EXISTS (
        SELECT 1 FROM linked_issues l JOIN owned_tree tree ON tree.id = l.issue_id
        WHERE l.question_id = q.id AND (tree.source_type = 'issue' OR asking.issue_id = @issue_id)
    )
    OR (asking.issue_id = @issue_id AND EXISTS (
        SELECT 1 FROM question_link l
        LEFT JOIN project p ON l.link_type = 'project' AND p.id = l.link_id AND p.workspace_id = @workspace_id
        LEFT JOIN autopilot a ON l.link_type = 'autopilot' AND a.id = l.link_id AND a.workspace_id = @workspace_id
        WHERE l.question_id = q.id AND l.workspace_id = @workspace_id
          AND ((p.lead_type = 'agent' AND p.lead_id = @agent_id)
            OR (a.assignee_type = 'agent' AND a.assignee_id = @agent_id))
    ))
  )
ORDER BY q.id;

-- name: HasActiveQuestionWakeTask :one
SELECT EXISTS (
    SELECT 1 FROM agent_task_queue t
    JOIN agent a ON a.id = t.agent_id AND a.workspace_id = @workspace_id
    JOIN issue i ON i.id = t.issue_id AND i.workspace_id = @workspace_id
    WHERE t.agent_id = @agent_id AND t.issue_id = @issue_id
      AND (t.status IN ('queued', 'dispatched', 'running', 'waiting_local_directory')
        OR (t.status = 'deferred' AND t.retry_of_task_id IS NOT NULL))
)::boolean AS has_active;
