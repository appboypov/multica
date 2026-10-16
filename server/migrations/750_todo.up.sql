CREATE TABLE IF NOT EXISTS todo (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL,
    member_id    UUID        NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN (
        'question', 'mention', 'assigned', 'agent_blocked', 'run_failed',
        'autopilot_paused', 'quota_reached', 'quick_create_failed', 'review'
    )),
    source_key   TEXT        NOT NULL,
    issue_id     UUID,
    question_id  UUID,
    comment_id   UUID,
    task_id      UUID,
    autopilot_id UUID,
    actor_type   TEXT        CHECK (actor_type IN ('member', 'agent')),
    actor_id     UUID,
    title        TEXT        NOT NULL,
    body         TEXT,
    details      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    done_at      TIMESTAMPTZ,
    done_by      TEXT        CHECK (done_by IN ('handled', 'member'))
);
