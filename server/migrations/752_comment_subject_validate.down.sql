-- Recreate the NOT VALID state migration 750 left; catalog-only.
ALTER TABLE comment DROP CONSTRAINT IF EXISTS comment_subject_check;
ALTER TABLE comment ADD CONSTRAINT comment_subject_check CHECK (
    (issue_id IS NULL) <> (release_id IS NULL)
) NOT VALID;
