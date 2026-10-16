-- WARNING: rollback retains only the first organisation, losing other bindings.
SET LOCAL lock_timeout = '2s';

ALTER TABLE member ADD COLUMN organisation_id UUID;
UPDATE member SET organisation_id = organisation_ids[1] WHERE role = 'client';
ALTER TABLE member
    DROP CONSTRAINT member_client_organisations_check,
    DROP COLUMN organisation_ids,
    ADD CONSTRAINT member_client_organisation_check CHECK ((role = 'client') = (organisation_id IS NOT NULL));

ALTER TABLE workspace_invitation ADD COLUMN organisation_id UUID;
UPDATE workspace_invitation SET organisation_id = organisation_ids[1] WHERE role = 'client';
ALTER TABLE workspace_invitation
    DROP CONSTRAINT workspace_invitation_client_organisations_check,
    DROP COLUMN organisation_ids,
    ADD CONSTRAINT workspace_invitation_client_organisation_check CHECK ((role = 'client') = (organisation_id IS NOT NULL));
