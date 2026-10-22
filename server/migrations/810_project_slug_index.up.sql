-- A project's slug is unique in its organisation: it names the project's
-- folder under the organisation's folder (ADR-0037). An organisation belongs
-- to one workspace, so the organisation id scopes it. The handler tells this
-- 409 apart by the index name.
-- Single statement + CONCURRENTLY (AGENTS.md).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_organisation_slug_unique
    ON project (organisation_id, slug);
