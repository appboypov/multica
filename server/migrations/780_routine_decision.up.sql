CREATE TABLE routine_decision (
    id TEXT PRIMARY KEY,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    input JSONB NOT NULL,
    task_id UUID,
    jev_confidence DOUBLE PRECISION,
    proposal JSONB,
    decider TEXT,
    question_id UUID,
    review_mode TEXT,
    applied_choice TEXT,
    work_task_id UUID,
    automatic_number BIGINT,
    reviews JSONB NOT NULL DEFAULT '[]'::jsonb,
    outcome TEXT NOT NULL DEFAULT 'judging',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
