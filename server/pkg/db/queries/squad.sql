-- name: CreateSquad :one
INSERT INTO squad (workspace_id, name, description, leader_id, creator_id, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetSquad :one
SELECT * FROM squad WHERE id = $1;

-- name: GetSquadInWorkspace :one
SELECT * FROM squad WHERE id = $1 AND workspace_id = $2;

-- name: LockSquadForAutopilotAssignment :one
-- Stabilizes the squad-to-leader resolution while an active Autopilot is
-- created, retargeted, or resumed. FOR SHARE conflicts with an ordinary
-- leader_id update, so the caller subsequently locks the same leader Agent
-- whose row Runtime teardown serializes against.
SELECT * FROM squad
WHERE id = $1 AND workspace_id = $2
FOR SHARE;

-- name: LockSquadForUpdate :one
-- Squad leader changes take the exclusive side of the same lock used by
-- Autopilot assignment. The handler then locks the proposed leader Agent and
-- pauses active squad Autopilots when that Agent is unbound.
SELECT * FROM squad
WHERE id = $1 AND workspace_id = $2
FOR UPDATE;

-- name: ListSquads :many
SELECT * FROM squad WHERE workspace_id = $1 AND archived_at IS NULL ORDER BY created_at ASC;

-- name: ListSquadMemberPreviewRows :many
-- Static squad membership summary for list/hover previews. This deliberately
-- excludes derived runtime/task status; the squad detail members-status
-- endpoint owns live state.
SELECT
    sm.squad_id,
    sm.member_type,
    sm.member_id,
    sm.role
FROM squad_member sm
JOIN squad s ON s.id = sm.squad_id
WHERE s.workspace_id = $1 AND s.archived_at IS NULL
ORDER BY
    sm.squad_id ASC,
    (sm.member_type = 'agent' AND sm.member_id = s.leader_id) DESC,
    sm.created_at ASC;

-- name: ListSquadMemberPreviewRowsBySquad :many
SELECT
    sm.squad_id,
    sm.member_type,
    sm.member_id,
    sm.role
FROM squad_member sm
JOIN squad s ON s.id = sm.squad_id
WHERE sm.squad_id = $1
ORDER BY
    (sm.member_type = 'agent' AND sm.member_id = s.leader_id) DESC,
    sm.created_at ASC;

-- name: ListAllSquads :many
SELECT * FROM squad WHERE workspace_id = $1 ORDER BY created_at ASC;

-- name: UpdateSquad :one
UPDATE squad SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    leader_id = COALESCE(sqlc.narg('leader_id'), leader_id),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    instructions = COALESCE(sqlc.narg('instructions'), instructions),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSquadRuntimeSkillExceptions :one
UPDATE squad
SET runtime_skill_exceptions = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSquadInheritRuntimeSkills :one
-- The section switch sets every runtime skill on or off, so the exceptions to
-- it start empty again.
UPDATE squad
SET inherit_runtime_skills = $2, runtime_skill_exceptions = '[]'::jsonb, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveSquad :one
UPDATE squad SET archived_at = now(), archived_by = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: AddSquadMember :one
INSERT INTO squad_member (squad_id, member_type, member_id, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RemoveSquadMember :execrows
DELETE FROM squad_member
WHERE squad_id = $1 AND member_type = $2 AND member_id = $3;

-- name: ListSquadMembers :many
SELECT * FROM squad_member WHERE squad_id = $1 ORDER BY created_at ASC;

-- name: UpdateSquadMemberRole :one
UPDATE squad_member SET role = $4
WHERE squad_id = $1 AND member_type = $2 AND member_id = $3
RETURNING *;

-- name: IsSquadMember :one
SELECT EXISTS(
    SELECT 1 FROM squad_member
    WHERE squad_id = $1 AND member_type = $2 AND member_id = $3
) AS is_member;

-- name: CountSquadMembers :one
SELECT count(*) FROM squad_member WHERE squad_id = $1;

-- name: GetSquadByAssignee :one
-- Look up the squad when an issue is assigned to a squad.
SELECT s.* FROM squad s WHERE s.id = $1 AND s.workspace_id = $2;

-- name: ListSquadsByMember :many
-- Find all squads a given entity belongs to in a workspace.
SELECT s.* FROM squad s
JOIN squad_member sm ON sm.squad_id = s.id
WHERE s.workspace_id = $1 AND sm.member_type = $2 AND sm.member_id = $3
ORDER BY s.created_at ASC;

-- name: LockSquadNesting :exec
-- Serializes every write that can close a squad loop or pair a sub-squad with
-- its main squad's leader (ADR-0017). A row lock on one squad cannot: two adds
-- on disjoint squads can close one loop together.
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg('workspace_id')::uuid::text || ':squad_nesting', 0));

-- name: ListSquadNestingMemberships :many
-- Every squad-in-squad membership in the workspace, archived squads included,
-- in the order they were added. The loop check walks them once per squad.
SELECT sm.squad_id, sm.member_id FROM squad_member sm
JOIN squad s ON s.id = sm.squad_id
WHERE s.workspace_id = sqlc.arg('workspace_id') AND sm.member_type = 'squad'
ORDER BY sm.created_at ASC, sm.id ASC;

-- name: ListSquadsNestedWithLeader :many
-- Non-archived squads led by leader_id that directly hold squad_id as a
-- member or that squad_id directly holds.
SELECT s.* FROM squad s
WHERE s.archived_at IS NULL
  AND s.leader_id = sqlc.arg('leader_id')
  AND (
    EXISTS (
        SELECT 1 FROM squad_member sm
        WHERE sm.member_type = 'squad' AND sm.squad_id = s.id AND sm.member_id = sqlc.arg('squad_id')
    )
    OR EXISTS (
        SELECT 1 FROM squad_member sm
        WHERE sm.member_type = 'squad' AND sm.squad_id = sqlc.arg('squad_id') AND sm.member_id = s.id
    )
  )
ORDER BY s.created_at ASC;

-- name: ListDelegatingParentSquads :many
-- Non-archived squads holding sub_squad_id as a member whose current leader,
-- in a leader run of that squad, posted a live comment on the issue that
-- @mentions the sub-squad. Their leaders take the sub-squad's report back.
SELECT s.* FROM squad s
JOIN squad_member sm ON sm.squad_id = s.id
WHERE s.workspace_id = sqlc.arg('workspace_id')
  AND s.archived_at IS NULL
  AND sm.member_type = 'squad'
  AND sm.member_id = sqlc.arg('sub_squad_id')
  AND EXISTS (
    SELECT 1 FROM comment c
    JOIN agent_task_queue t ON t.id = c.source_task_id
    WHERE c.issue_id = sqlc.arg('issue_id')
      AND c.deleted_at IS NULL
      AND t.is_leader_task
      AND t.squad_id = s.id
      AND t.agent_id = s.leader_id
      AND strpos(c.content, 'mention://squad/' || sqlc.arg('sub_squad_id')::uuid::text) > 0
  )
ORDER BY s.created_at ASC;

-- name: TransferSquadAssignees :exec
-- Transfer all issues assigned to a squad to the squad's leader agent.
UPDATE issue SET assignee_type = 'agent', assignee_id = $2, revision = revision + 1, updated_at = now()
WHERE assignee_type = 'squad' AND assignee_id = $1;

-- name: TransferSquadAutopilotsToLeader :exec
-- Mirrors TransferSquadAssignees for autopilot rows: when a squad is archived,
-- any autopilot still pointing at the squad would otherwise dangle and the
-- admission gate would skip every subsequent dispatch with "assignee squad
-- cannot be resolved". Rewrite the assignee in place to the leader agent so
-- the autopilot keeps firing under the same leader-only execution semantics
-- it had a moment before the archive (Path A from MUL-2429).
UPDATE autopilot
SET assignee_type = 'agent',
    assignee_id = $2,
    updated_at = now()
WHERE assignee_type = 'squad' AND assignee_id = $1;

-- name: ListIssueContributingSquads :many
-- The squads that reach every run on an issue (ADR-0034): the live squad
-- assigned to the issue or, when it has none, to its closest ancestor issue,
-- plus every live squad holding that squad through nested membership, each
-- once. Outermost first: a squad sorts by its longest membership path down to
-- the owning squad, so every parent precedes the squads it holds and the
-- owning squad comes last. Archived squads count as absent, and the path
-- arrays keep a malformed issue cycle or membership loop finite.
WITH RECURSIVE issue_chain AS (
    SELECT i.id, i.parent_issue_id, i.assignee_type, i.assignee_id, 0 AS distance, ARRAY[i.id] AS path
    FROM issue i
    WHERE i.id = sqlc.arg('issue_id') AND i.workspace_id = sqlc.arg('workspace_id')
    UNION ALL
    SELECT parent.id, parent.parent_issue_id, parent.assignee_type, parent.assignee_id, c.distance + 1, c.path || parent.id
    FROM issue_chain c
    JOIN issue parent ON parent.id = c.parent_issue_id AND parent.workspace_id = sqlc.arg('workspace_id')
    WHERE NOT parent.id = ANY(c.path)
), owner AS (
    SELECT s.id
    FROM issue_chain c
    JOIN squad s ON s.id = c.assignee_id AND s.workspace_id = sqlc.arg('workspace_id') AND s.archived_at IS NULL
    WHERE c.assignee_type = 'squad'
    ORDER BY c.distance
    LIMIT 1
), squad_tree AS (
    SELECT o.id AS squad_id, 0 AS depth, ARRAY[o.id] AS path FROM owner o
    UNION ALL
    SELECT parent.id, t.depth + 1, t.path || parent.id
    FROM squad_tree t
    JOIN squad_member sm ON sm.member_type = 'squad' AND sm.member_id = t.squad_id
    JOIN squad parent ON parent.id = sm.squad_id AND parent.workspace_id = sqlc.arg('workspace_id') AND parent.archived_at IS NULL
    WHERE NOT parent.id = ANY(t.path)
)
SELECT s.* FROM squad s
JOIN (SELECT squad_id, max(depth) AS depth FROM squad_tree GROUP BY squad_id) d ON d.squad_id = s.id
ORDER BY d.depth DESC, s.created_at ASC, s.id ASC;

-- name: ListSquadMemberStatusRows :many
-- Per-row join used to build the squad-members status view. One row per
-- (squad_member × in_flight_task); members with no in-flight task return a
-- single row with NULL task_* columns. Human members and agent members
-- with no agent row also return one row with NULL agent_/runtime_ columns.
-- waiting_local_directory stays in the row set so its issue remains visible,
-- but the handler only treats dispatched/running rows as working because the
-- squad status vocabulary has no queued bucket.
SELECT
    sm.id              AS squad_member_id,
    sm.member_type     AS member_type,
    sm.member_id       AS member_id,
    a.archived_at      AS agent_archived_at,
    ar.status          AS runtime_status,
    ar.last_seen_at    AS runtime_last_seen_at,
    atq.id             AS task_id,
    atq.status         AS task_status,
    atq.issue_id       AS task_issue_id,
    atq.dispatched_at  AS task_dispatched_at,
    i.number           AS issue_number,
    i.title            AS issue_title,
    i.status           AS issue_status
FROM squad_member sm
LEFT JOIN agent a
       ON sm.member_type = 'agent' AND a.id = sm.member_id
LEFT JOIN agent_runtime ar
       ON ar.id = a.runtime_id
LEFT JOIN agent_task_queue atq
       ON sm.member_type = 'agent'
      AND atq.agent_id = sm.member_id
      AND atq.status IN ('dispatched', 'running', 'waiting_local_directory')
LEFT JOIN issue i
       ON i.id = atq.issue_id
WHERE sm.squad_id = $1
ORDER BY sm.created_at ASC, atq.dispatched_at DESC NULLS LAST;
