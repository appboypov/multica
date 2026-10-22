-- A project's slug names its folder under its organisation's folder in the
-- work root (ADR-0037): {root}/{organisation}/{slug}/repos/{repo}. It follows
-- the organisation key's folder-name rule and is unique per organisation
-- (index in 810). A project that holds repositories needs one; the handler
-- checks that, so the column stays nullable.
--
-- Existing projects get the kebab-case of their title, deduplicated per
-- organisation with -2, -3, ... in creation order. A title without a letter
-- or digit becomes "project".
--
-- The runner sends this file as one implicit transaction. ALTER TABLE takes a
-- short ACCESS EXCLUSIVE lock on project, so bound the wait and retry on the
-- next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

ALTER TABLE project
    ADD COLUMN IF NOT EXISTS slug TEXT CHECK (slug IS NULL OR slug ~ '^[a-z0-9][a-z0-9-]*$');

DO $$
DECLARE
    p         RECORD;
    base      TEXT;
    candidate TEXT;
    n         INT;
BEGIN
    FOR p IN
        SELECT id, workspace_id, organisation_id, title
        FROM project
        WHERE slug IS NULL
        ORDER BY created_at, id
    LOOP
        base := trim(both '-' from regexp_replace(lower(p.title), '[^a-z0-9]+', '-', 'g'));
        IF base = '' THEN
            base := 'project';
        END IF;
        candidate := base;
        n := 1;
        WHILE EXISTS (
            SELECT 1 FROM project other
            WHERE other.workspace_id = p.workspace_id
              AND other.organisation_id IS NOT DISTINCT FROM p.organisation_id
              AND other.slug = candidate
        ) LOOP
            n := n + 1;
            candidate := base || '-' || n;
        END LOOP;
        UPDATE project SET slug = candidate WHERE id = p.id;
    END LOOP;
END
$$;
