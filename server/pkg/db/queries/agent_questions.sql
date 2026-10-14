-- name: GetIssueAgentQuestions :one
-- The issue's Agent questions and its workspace's settings, read once when a
-- task on the issue is claimed (ADR-0005).
SELECT i.agent_questions, w.settings
FROM issue i
JOIN workspace w ON w.id = i.workspace_id
WHERE i.id = $1;

-- name: CountOwnAgentQuestionIssues :one
-- Open issues with their own Agent questions setting, for the workspace
-- Questions settings.
SELECT count(*)
FROM issue
WHERE workspace_id = @workspace_id
  AND agent_questions IS NOT NULL
  AND NOT (status = ANY(@terminal_status_keys::text[]));
