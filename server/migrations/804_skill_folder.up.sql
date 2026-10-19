-- An agent's runtime_skill_exceptions are the runtime skills that differ from
-- its section switch (inherit_runtime_skills): with the switch on they are the
-- skills it leaves out, with the switch off the skills it takes (ADR-0033).
ALTER TABLE agent RENAME COLUMN disabled_runtime_skills TO runtime_skill_exceptions;

-- A skill folder is a folder on one machine (the daemon id) whose skills are
-- in the workspace's library. Each of its skills is a skill row whose
-- config.origin names the folder; a run reads the skill's files from the
-- folder on that machine. No foreign keys or cascades (AGENTS.md): removing a
-- folder deletes its skill rows in the same transaction, and workspace
-- teardown deletes the folders.
CREATE TABLE IF NOT EXISTS skill_folder (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL,
    daemon_id    TEXT        NOT NULL,
    path         TEXT        NOT NULL,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
