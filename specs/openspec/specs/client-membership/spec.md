# client-membership Specification

## Purpose
Client contacts keep one organisation-bound membership through invitation acceptance, login and staff management.

## Requirements

### Requirement: A client membership is bound to one organisation of its workspace
A workspace membership with the role `client` SHALL be bound to exactly one organisation of the same workspace, and Multica MUST NOT create a client membership or a client invitation without such an organisation.

#### Scenario: A client invitation names its organisation
- **GIVEN** the workspace holds the organisation Rebl Records
- **WHEN** Brian, an admin, invites rizgar@rebl.nl with the role `client` for Rebl Records
- **THEN** the invitation shows the role `client` and the organisation Rebl Records

#### Scenario: A client invitation without an organisation
- **WHEN** Brian invites rizgar@rebl.nl with the role `client` and no organisation
- **THEN** the invitation is refused and no invitation exists for rizgar@rebl.nl

#### Scenario: An organisation of another workspace
- **GIVEN** the organisation Flextender belongs to another workspace
- **WHEN** Brian invites rizgar@rebl.nl with the role `client` for Flextender
- **THEN** the invitation is refused and no invitation exists for rizgar@rebl.nl

### Requirement: An owner or admin invites a named client contact
An owner or admin SHALL invite a client contact by email address for one organisation through the existing named invitation, from the API and from `multica workspace member invite --role client` with that organisation, and a member MUST NOT invite a client.

#### Scenario: Inviting from the CLI
- **GIVEN** Brian is an admin of the workspace that holds Rebl Records
- **WHEN** he runs `multica workspace member invite rizgar@rebl.nl --role client` with the organisation Rebl Records
- **THEN** rizgar@rebl.nl has a pending invitation with the role `client` for Rebl Records

#### Scenario: The CLI without an organisation
- **WHEN** Brian runs `multica workspace member invite rizgar@rebl.nl --role client` without an organisation
- **THEN** the command fails and says a client invitation needs an organisation
- **AND** no invitation exists for rizgar@rebl.nl

#### Scenario: A member cannot invite a client
- **GIVEN** Sam is a member, not an owner or admin
- **WHEN** Sam invites rizgar@rebl.nl with the role `client` for Rebl Records
- **THEN** the invitation is refused

### Requirement: Accepting and logging in keep the client role and its organisation
Accepting a client invitation SHALL create a membership with the role `client` and the invitation's organisation, and that membership SHALL stay `client` with the same organisation at every login.

#### Scenario: A client accepts an invitation
- **GIVEN** rizgar@rebl.nl has a pending client invitation for Rebl Records
- **WHEN** Rizgar logs in and accepts it
- **THEN** Rizgar's membership has the role `client` and the organisation Rebl Records

#### Scenario: A later login
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar logs out and logs in again
- **THEN** Rizgar's membership still has the role `client` and the organisation Rebl Records

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
A client whose organisation is missing, deleted or of another workspace MUST NOT gain workspace access, and accepting a client invitation whose organisation no longer exists MUST NOT create a membership.

#### Scenario: The organisation is deleted after acceptance
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rebl Records is deleted and Rizgar opens the workspace
- **THEN** Rizgar's workspace access is refused

#### Scenario: The organisation is deleted before acceptance
- **GIVEN** rizgar@rebl.nl has a pending client invitation for Rebl Records
- **WHEN** Rebl Records is deleted and Rizgar accepts the invitation
- **THEN** the acceptance is refused and Rizgar has no membership of the workspace

### Requirement: A client sees their own role and organisation
The current-membership response SHALL show a client the role `client` and the organisation the membership is bound to.

#### Scenario: Rizgar checks his membership
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar asks for his current membership of the workspace
- **THEN** the response shows the role `client` and the organisation Rebl Records

### Requirement: A client logs in and out
A client SHALL log in and log out the same way a member does.

#### Scenario: Rizgar logs out
- **GIVEN** Rizgar is logged in as a client of Rebl Records
- **WHEN** Rizgar logs out
- **THEN** his session ends and a request with that session is refused as unauthenticated

### Requirement: An owner or admin manages a client membership
An owner or admin SHALL change a client's organisation to another organisation of the workspace, remove a client membership, and change a member to a client only together with an organisation.

#### Scenario: Moving a client to another organisation
- **GIVEN** Rizgar is a client of Rebl Records and the workspace holds Buttler
- **WHEN** Brian changes Rizgar's organisation to Buttler
- **THEN** Rizgar's current membership shows the organisation Buttler

#### Scenario: Removing a client
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Brian removes Rizgar's membership
- **THEN** Rizgar has no membership of the workspace

#### Scenario: Changing a member to a client
- **GIVEN** Sam is a member
- **WHEN** Brian changes Sam's role to `client` for Rebl Records
- **THEN** Sam's membership has the role `client` and the organisation Rebl Records

#### Scenario: Changing a member to a client without an organisation
- **GIVEN** Sam is a member
- **WHEN** Brian changes Sam's role to `client` without an organisation
- **THEN** the change is refused and Sam stays a member

### Requirement: An issue can be assigned to a client contact
An owner, admin or member SHALL assign an issue to a client contact through the existing member assignee, identified by the contact's user ID.

#### Scenario: Brian puts a wait on Rizgar's name
- **GIVEN** Rizgar is a client of Rebl Records and WORK-12 has no assignee
- **WHEN** Brian assigns WORK-12 to Rizgar
- **THEN** WORK-12's assignee is the member Rizgar
