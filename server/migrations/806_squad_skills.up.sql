-- A squad carries the same skill model as an agent: workspace skills it is
-- assigned (squad_skill, each with its own enabled flag), a runtime skill
-- section switch and the runtime skills that differ from that switch. Unlike
-- an agent's switch, a squad's defaults to off so a new squad gives no runtime
-- skills. No foreign keys or cascades (AGENTS.md): deleting a skill deletes its
-- squad_skill rows in the same statement, and workspace teardown deletes the
-- rows of its squads and skills.
ALTER TABLE squad ADD COLUMN inherit_runtime_skills BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE squad ADD COLUMN runtime_skill_exceptions JSONB NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS squad_skill (
    squad_id   UUID        NOT NULL,
    skill_id   UUID        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (squad_id, skill_id)
);
