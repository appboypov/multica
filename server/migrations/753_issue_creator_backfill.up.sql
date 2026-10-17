-- Keep the run as provenance while crediting the member it worked for.
-- The credited member follows the issue as its creator, so they get a
-- creator's notices: an active 'delegated' row becomes 'creator', the same
-- upgrade AddIssueSubscriber applies, and a member without a row gets one.
-- Issues from before the delegated rule have no row. An opt-out wins: a
-- tombstone on the issue, or a 'subtree' tombstone on an ancestor, keeps the
-- member off it, the same walk AddDelegatedSubscriber makes.
WITH RECURSIVE credited AS (
    UPDATE issue AS i
    SET creator_type = 'member',
        creator_id = task.originator_user_id
    FROM agent_task_queue AS task
    WHERE i.creator_type = 'agent'
      AND i.origin_type IN ('agent_create', 'quick_create')
      AND i.origin_id = task.id
      AND task.originator_user_id IS NOT NULL
    RETURNING i.id, i.creator_id, i.parent_issue_id
),
ancestors(credited_id, node_id, parent_id, depth) AS (
    SELECT c.id, c.id, c.parent_issue_id, 0 FROM credited c
    UNION ALL
    SELECT a.credited_id, i.id, i.parent_issue_id, a.depth + 1
    FROM issue i JOIN ancestors a ON i.id = a.parent_id
)
INSERT INTO issue_subscriber (issue_id, user_type, user_id, reason)
SELECT c.id, 'member', c.creator_id, 'creator'
FROM credited c
WHERE NOT EXISTS (
    SELECT 1
    FROM ancestors a
    JOIN issue_subscriber s ON s.issue_id = a.node_id
    WHERE a.credited_id = c.id
      AND s.user_type = 'member' AND s.user_id = c.creator_id
      AND s.unsubscribed_at IS NOT NULL
      AND (a.depth = 0 OR s.opt_out_scope = 'subtree')
)
ON CONFLICT (issue_id, user_type, user_id) DO UPDATE
SET reason = EXCLUDED.reason
WHERE issue_subscriber.unsubscribed_at IS NULL
  AND issue_subscriber.reason = 'delegated';
