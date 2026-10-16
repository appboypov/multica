# client-membership Specification

## Purpose
Client contacts keep one organisation-bound membership through invitation acceptance, login and staff management.

## Requirements

### Requirement: A client membership is bound to organisations of its workspace
A workspace membership with role `client` SHALL hold a set of one or more organisations of the same workspace, and Multica MUST NOT create a client membership or invitation without such a set; staff memberships and invitations SHALL hold no organisations.

#### Scenario: A client invitation names its organisation
- **GIVEN** organisations A and B belong to this workspace
- **WHEN** Brian invites a contact with role `client` for A and B
- **THEN** one invitation shows both organisations

#### Scenario: A client invitation without an organisation
- **GIVEN** Brian is an admin
- **WHEN** Brian invites a client with no organisations or an empty set
- **THEN** the invitation is refused and no invitation is created

#### Scenario: An organisation of another workspace
- **GIVEN** A belongs to this workspace and B belongs to another
- **WHEN** Brian invites a client for A and B
- **THEN** the complete invitation is refused without a partial invitation

#### Scenario: Staff have no organisations
- **GIVEN** Brian is an admin
- **WHEN** he invites staff with a non-empty organisation set
- **THEN** the invitation is refused

#### Scenario: Duplicate organisations are one entry
- **GIVEN** A belongs to this workspace
- **WHEN** Brian invites a client with A twice
- **THEN** the invitation contains A once

### Requirement: An owner or admin invites a named client contact
An owner or admin SHALL invite a named client contact for one or more organisations through the API and `multica workspace member invite --role client` with repeatable `--organisation`, and a member MUST NOT invite a client.

#### Scenario: Inviting from the CLI
- **GIVEN** Brian is an admin and A and B belong to the workspace
- **WHEN** he invites a contact with `--role client --organisation a --organisation b`
- **THEN** one pending invitation contains A and B

#### Scenario: The CLI without an organisation
- **GIVEN** Brian is an admin
- **WHEN** he invites a client without `--organisation`
- **THEN** the command fails and no invitation is created

#### Scenario: A member cannot invite a client
- **GIVEN** Sam is a member, not an owner or admin
- **WHEN** Sam invites a client for A and B
- **THEN** the invitation is refused

### Requirement: Accepting and logging in keep the client role and its organisations
Accepting a client invitation SHALL create one client membership with every invited organisation that still exists in the workspace, and later login SHALL preserve the role and organisation set.

#### Scenario: A client accepts an invitation
- **GIVEN** a contact has a pending client invitation for A and B
- **WHEN** the contact accepts
- **THEN** one client membership contains A and B

#### Scenario: A later login
- **GIVEN** a client membership contains A and B
- **WHEN** the client logs out and logs in again
- **THEN** the membership still has role client and organisations A and B

#### Scenario: Some invited organisations were deleted
- **GIVEN** an invitation names A and B and A is deleted
- **WHEN** the contact accepts
- **THEN** one client membership contains only B

### Requirement: A client is never upgraded as a fallback
Multica MUST NOT give a client the role member, admin or owner through login, invitation acceptance, a share link or any fallback; only an explicit role change by an owner or admin changes a client's role.

#### Scenario: A client opens a share link
- **GIVEN** Rizgar is a client of Rebl Records and the workspace has a share link that grants the role member
- **WHEN** Rizgar opens the share link
- **THEN** Rizgar's membership still has the role `client` and the organisation Rebl Records

#### Scenario: A share link cannot grant the client role
- **WHEN** Brian creates a share link with the role `client`
- **THEN** the share link is refused

### Requirement: A client without a valid organisation has no workspace access
A client without any existing same-workspace organisation MUST NOT gain workspace access, and accepting an invitation without any surviving same-workspace organisation MUST NOT create a membership.

#### Scenario: One organisation is deleted after acceptance
- **GIVEN** a client belongs to A and B
- **WHEN** A is deleted
- **THEN** the membership retains B and the current-membership response shows B

#### Scenario: The organisation is deleted after acceptance
- **GIVEN** a client belongs to A and B
- **WHEN** both are deleted and the client opens the workspace
- **THEN** workspace access is refused

#### Scenario: The organisation is deleted before acceptance
- **GIVEN** a pending invitation names A and B and both are deleted
- **WHEN** the contact accepts
- **THEN** acceptance is refused and no membership is created

### Requirement: A client sees their own role and organisations
The current-membership response SHALL show role `client` and every existing same-workspace organisation of the membership, and member and invitation lists SHALL show every organisation of a client.

#### Scenario: Rizgar checks his membership
- **GIVEN** a client belongs to A and B
- **WHEN** the client asks for current memberships
- **THEN** the response contains role client and organisations A and B

#### Scenario: Staff checks member and invitation lists
- **GIVEN** a client membership names A and B and a pending client invitation names B and C
- **WHEN** an admin lists members and invitations
- **THEN** the member shows A and B and the invitation shows B and C

### Requirement: A client logs in and out
A client SHALL log in and log out the same way a member does.

#### Scenario: Rizgar logs out
- **GIVEN** Rizgar is logged in as a client of Rebl Records
- **WHEN** Rizgar logs out
- **THEN** his session ends and a request with that session is refused as unauthenticated

### Requirement: An owner or admin manages a client membership
An owner or admin SHALL add and remove organisations through the existing membership update, refuse removal of the last organisation, remove a membership to revoke all access, and change staff to client only with at least one same-workspace organisation.

#### Scenario: Moving a client to another organisation
- **GIVEN** a client belongs to A and B and C belongs to the workspace
- **WHEN** Brian adds C and removes A
- **THEN** the membership shows B and C

#### Scenario: Removing the last organisation
- **GIVEN** a client belongs only to C
- **WHEN** Brian updates the membership with an empty organisation set
- **THEN** the update is refused and C remains

#### Scenario: Removing a client
- **GIVEN** a client belongs to A and B
- **WHEN** Brian removes the membership
- **THEN** the contact has no membership of the workspace

#### Scenario: Changing a member to a client
- **GIVEN** Sam is a member
- **WHEN** Brian changes Sam to client with A and B
- **THEN** Sam has one client membership containing A and B

#### Scenario: Changing a member to a client without an organisation
- **GIVEN** Sam is a member
- **WHEN** Brian changes Sam to client without organisations
- **THEN** the update is refused and Sam remains a member

### Requirement: An issue can be assigned to a client contact
An owner, admin or member SHALL assign an issue to a client contact through the existing member assignee, identified by the contact's user ID.

#### Scenario: Brian puts a wait on Rizgar's name
- **GIVEN** Rizgar is a client of Rebl Records and WORK-12 has no assignee
- **WHEN** Brian assigns WORK-12 to Rizgar
- **THEN** WORK-12's assignee is the member Rizgar

### Requirement: Existing organisation bindings survive migration
Existing client memberships and pending invitations SHALL retain their one organisation as the sole entry of their organisation set, and the single-organisation columns SHALL be absent after migration.

#### Scenario: Existing client and invitation migrate
- **GIVEN** a client membership and a pending invitation each contain a single organisation before migration
- **WHEN** the migration runs
- **THEN** both contain their original organisation as their only set entry
- **AND** the single-organisation columns are absent
