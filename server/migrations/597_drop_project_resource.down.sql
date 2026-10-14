-- Rollback recreates the empty resource table. The indexes are ordinary
-- CREATE INDEX because no resource rows survive the cutover.
CREATE TABLE project_resource (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    resource_type TEXT NOT NULL,
    resource_ref JSONB NOT NULL,
    label TEXT,
    position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    UNIQUE (project_id, resource_type, resource_ref)
);
CREATE INDEX idx_project_resource_project ON project_resource(project_id, position);
CREATE INDEX idx_project_resource_workspace ON project_resource(workspace_id);
