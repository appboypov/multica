-- Validate the CHECK migration 750 added NOT VALID, under SHARE UPDATE
-- EXCLUSIVE. Every existing comment has an issue and no release.
ALTER TABLE comment VALIDATE CONSTRAINT comment_subject_check;
