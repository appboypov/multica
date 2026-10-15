SET LOCAL lock_timeout = '2s';

-- Rollback removes memberships and invitations unsupported by the staff-only schema.
DELETE FROM member WHERE role = 'client';
DELETE FROM workspace_invitation WHERE role = 'client';

ALTER TABLE member
    DROP CONSTRAINT member_client_organisation_check,
    DROP CONSTRAINT member_role_check,
    DROP COLUMN organisation_id,
    ADD CONSTRAINT member_role_check CHECK (role IN ('owner', 'admin', 'member'));

ALTER TABLE workspace_invitation
    DROP CONSTRAINT workspace_invitation_client_organisation_check,
    DROP CONSTRAINT workspace_invitation_role_check,
    DROP COLUMN organisation_id,
    ADD CONSTRAINT workspace_invitation_role_check CHECK (role IN ('admin', 'member'));
