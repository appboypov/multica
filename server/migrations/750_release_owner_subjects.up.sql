-- Native release subjects (release-owner). A release may have one optional
-- owner, an agent or a squad of its workspace; existing releases stay unowned.
-- Coordination of a release reuses the shared stores: its runs are
-- agent_task_queue rows, its discussion is comment rows and its waits are
-- issue_wakeup rows, each carrying release_id instead of issue_id. All
-- references are soft: the application validates and deletes dependents.
--
-- release is small and the new owner columns are NULL for every row, so its
-- CHECK is validated here. issue_wakeup holds rules, not events, and is
-- validated here too. comment and agent_task_queue are hot tables: their
-- CHECKs are added NOT VALID (enforced on every write from here on) and
-- validated in 751/752 under SHARE UPDATE EXCLUSIVE, as 483/489 do.
ALTER TABLE release
    ADD COLUMN IF NOT EXISTS owner_type text,
    ADD COLUMN IF NOT EXISTS owner_id uuid;
ALTER TABLE release DROP CONSTRAINT IF EXISTS release_owner_pair_check;
ALTER TABLE release ADD CONSTRAINT release_owner_pair_check CHECK (
    (owner_type IS NULL AND owner_id IS NULL)
    OR (owner_type IN ('agent', 'squad') AND owner_id IS NOT NULL)
);

-- A release run has release_id and none of the other subjects; every existing
-- variant (issue, chat, autopilot, quick-create) keeps release_id NULL and its
-- current rules.
ALTER TABLE agent_task_queue ADD COLUMN IF NOT EXISTS release_id uuid;
ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_queue_release_subject_check;
ALTER TABLE agent_task_queue ADD CONSTRAINT agent_task_queue_release_subject_check CHECK (
    release_id IS NULL
    OR (issue_id IS NULL AND chat_session_id IS NULL AND autopilot_run_id IS NULL)
) NOT VALID;

-- A comment belongs to exactly one subject: an issue or a release.
ALTER TABLE comment ADD COLUMN IF NOT EXISTS release_id uuid;
ALTER TABLE comment ALTER COLUMN issue_id DROP NOT NULL;
ALTER TABLE comment DROP CONSTRAINT IF EXISTS comment_subject_check;
ALTER TABLE comment ADD CONSTRAINT comment_subject_check CHECK (
    (issue_id IS NULL) <> (release_id IS NULL)
) NOT VALID;

-- A wakeup rule belongs to exactly one subject. release_coordination is the
-- platform rule of an owned release: it has no fixed agent and targets the
-- release's current owner (a squad's leader) when it fires. child_done stays
-- an issue rule.
ALTER TABLE issue_wakeup ADD COLUMN IF NOT EXISTS release_id uuid;
ALTER TABLE issue_wakeup ALTER COLUMN issue_id DROP NOT NULL;
ALTER TABLE issue_wakeup DROP CONSTRAINT IF EXISTS issue_wakeup_subject_check;
ALTER TABLE issue_wakeup ADD CONSTRAINT issue_wakeup_subject_check CHECK (
    (issue_id IS NULL) <> (release_id IS NULL)
);
ALTER TABLE issue_wakeup DROP CONSTRAINT IF EXISTS issue_wakeup_system_rule_check;
ALTER TABLE issue_wakeup ADD CONSTRAINT issue_wakeup_system_rule_check CHECK (
    system_rule IS NULL
    OR (system_rule = 'child_done' AND issue_id IS NOT NULL)
    OR (system_rule = 'release_coordination' AND release_id IS NOT NULL)
);
