-- Organisation bindings are soft references, validated in the membership handlers.
-- Bound ACCESS EXCLUSIVE lock waits so a busy table does not queue writers.
SET LOCAL lock_timeout = '2s';

ALTER TABLE member
    ADD COLUMN organisation_id UUID,
    DROP CONSTRAINT member_role_check,
    ADD CONSTRAINT member_role_check CHECK (role IN ('owner', 'admin', 'member', 'client')),
    ADD CONSTRAINT member_client_organisation_check CHECK ((role = 'client') = (organisation_id IS NOT NULL));

ALTER TABLE workspace_invitation
    ADD COLUMN organisation_id UUID,
    DROP CONSTRAINT workspace_invitation_role_check,
    ADD CONSTRAINT workspace_invitation_role_check CHECK (role IN ('admin', 'member', 'client')),
    ADD CONSTRAINT workspace_invitation_client_organisation_check CHECK ((role = 'client') = (organisation_id IS NOT NULL));
