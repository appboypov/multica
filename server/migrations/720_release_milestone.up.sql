-- Free named calendar-day milestones of a release (ADR-0010).
-- Relationships are validated and deleted by the application; no foreign keys.
CREATE TABLE IF NOT EXISTS release_milestone (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    release_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    date DATE NOT NULL CHECK (isfinite(date)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
