-- The daemon reports the branch, while the database owns the done date
-- across all issue status writers (ADR-0014, ADR-0015).
ALTER TABLE worklane ADD COLUMN branch text;
ALTER TABLE issue ADD COLUMN done_at timestamptz;

UPDATE issue i
SET done_at = COALESCE((
    SELECT a.created_at FROM activity_log a
    WHERE a.workspace_id = i.workspace_id AND a.issue_id = i.id
      AND a.action = 'status_changed' AND a.details ->> 'to' = i.status
    ORDER BY a.created_at DESC, a.id DESC LIMIT 1
), i.updated_at)
WHERE issue_effective_status(i.workspace_id, i.status) = 'done';

CREATE FUNCTION issue_done_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF issue_effective_status(NEW.workspace_id, NEW.status) = 'done' THEN
        IF TG_OP = 'UPDATE' AND issue_effective_status(OLD.workspace_id, OLD.status) = 'done' THEN
            NEW.done_at := OLD.done_at;
        ELSE
            NEW.done_at := now();
        END IF;
    ELSE
        NEW.done_at := NULL;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER trg_issue_done_at BEFORE INSERT OR UPDATE OF status ON issue
    FOR EACH ROW EXECUTE FUNCTION issue_done_at();
