ALTER TABLE todo DROP CONSTRAINT IF EXISTS todo_kind_check;
ALTER TABLE todo ADD CONSTRAINT todo_kind_check CHECK (kind IN (
    'question', 'mention', 'assigned', 'agent_blocked', 'run_failed',
    'autopilot_paused', 'quota_reached', 'quick_create_failed', 'review',
    'check', 'trust_proposal'
));

CREATE TABLE IF NOT EXISTS check_settings (
    workspace_id UUID PRIMARY KEY,
    streak_length INTEGER NOT NULL DEFAULT 5 CHECK (streak_length BETWEEN 1 AND 100),
    sample_every INTEGER NOT NULL DEFAULT 5 CHECK (sample_every BETWEEN 1 AND 100)
);

CREATE TABLE IF NOT EXISTS project_check_trust (
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('data_model', 'data_flow', 'design', 'prototype', 'feature', 'improvement', 'agent_tool', 'release')),
    member_id UUID NOT NULL,
    passed INTEGER NOT NULL DEFAULT 0,
    feedback INTEGER NOT NULL DEFAULT 0,
    automatic_passed INTEGER NOT NULL DEFAULT 0,
    streak INTEGER NOT NULL DEFAULT 0,
    automatic BOOLEAN NOT NULL DEFAULT false CHECK (NOT automatic OR kind <> 'release'),
    automatic_opens INTEGER NOT NULL DEFAULT 0,
    proposal_id UUID,
    PRIMARY KEY (workspace_id, project_id, kind)
);
