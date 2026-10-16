-- Row-owned soft references; handlers validate every organisation's workspace.
SET LOCAL lock_timeout = '2s';

ALTER TABLE member ADD COLUMN organisation_ids UUID[] NOT NULL DEFAULT '{}';
UPDATE member SET organisation_ids = ARRAY[organisation_id] WHERE role = 'client';
ALTER TABLE member
    DROP CONSTRAINT member_client_organisation_check,
    DROP COLUMN organisation_id,
    ADD CONSTRAINT member_client_organisations_check CHECK (
        CASE WHEN role = 'client' THEN cardinality(organisation_ids) > 0
             ELSE cardinality(organisation_ids) = 0 END
        AND array_position(organisation_ids, NULL) IS NULL
    );

ALTER TABLE workspace_invitation ADD COLUMN organisation_ids UUID[] NOT NULL DEFAULT '{}';
UPDATE workspace_invitation SET organisation_ids = ARRAY[organisation_id] WHERE role = 'client';
ALTER TABLE workspace_invitation
    DROP CONSTRAINT workspace_invitation_client_organisation_check,
    DROP COLUMN organisation_id,
    ADD CONSTRAINT workspace_invitation_client_organisations_check CHECK (
        CASE WHEN role = 'client' THEN cardinality(organisation_ids) > 0
             ELSE cardinality(organisation_ids) = 0 END
        AND array_position(organisation_ids, NULL) IS NULL
    );
