CREATE TABLE routine_decision (
    id TEXT PRIMARY KEY,
    workspace_id UUID NOT NULL,
    issue_id UUID NOT NULL,
    input JSONB NOT NULL,
    task_id UUID,
    outcome TEXT NOT NULL DEFAULT 'judging',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
