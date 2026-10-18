# staff-client-controls Specification

## Purpose
Staff invite client contacts for organisations and assign them issues through the shared member and assignee controls.

## Requirements

### Requirement: Staff invite a client for organisations
Invite member SHALL offer the role Client next to Member and Admin, and with Client chosen it MUST ask for one or more organisations of the workspace before the invitation can be sent.

#### Scenario: Brian invites a client for two organisations
- **GIVEN** organisations IVGA Vastgoed and Whitebird belong to the workspace
- **WHEN** Brian invites nasir@ivga.nl with role Client for IVGA Vastgoed and Whitebird
- **THEN** Pending invitations lists nasir@ivga.nl with role Client and both organisations

#### Scenario: A client invitation without organisations
- **GIVEN** Brian chose role Client
- **WHEN** no organisation is chosen
- **THEN** the invitation can't be sent

#### Scenario: A staff invitation has no organisations
- **GIVEN** Brian chose Member or Admin
- **WHEN** he sends the invitation
- **THEN** no Organisations field shows and the invitation holds no organisations

#### Scenario: Share links stay staff only
- **GIVEN** Brian opens the share link settings
- **WHEN** he picks the role of the link
- **THEN** only Member and Admin are offered

### Requirement: Staff see a client's role and organisations
Members, Pending invitations and the member profile SHALL show the role Client for a client, and Members and Pending invitations SHALL show a badge for each organisation of that client.

#### Scenario: A client member row
- **GIVEN** Nasir is a client for IVGA Vastgoed and Whitebird
- **WHEN** Brian opens Members
- **THEN** Nasir's row shows role Client and a badge for each organisation

#### Scenario: The member profile of a client
- **GIVEN** Nasir is a client
- **WHEN** Brian opens Nasir's profile
- **THEN** the role badge reads Client

#### Scenario: A response from an older server
- **GIVEN** a member response holds no organisations or an unknown role
- **WHEN** the members list loads
- **THEN** the member shows no organisation badges and an unknown role reads as Member

### Requirement: Staff change a client's organisations
A client's row menu SHALL hold Organisations, which lists every organisation of the workspace with the client's own checked; a click MUST add or remove that one organisation, and the last organisation MUST NOT be removable.

#### Scenario: Brian removes an organisation
- **GIVEN** Nasir is a client for IVGA Vastgoed and Whitebird
- **WHEN** Brian unticks Whitebird under Organisations
- **THEN** Nasir's row shows only IVGA Vastgoed and it stays so after a reload

#### Scenario: Brian adds an organisation
- **GIVEN** Nasir is a client for IVGA Vastgoed
- **WHEN** Brian ticks Whitebird under Organisations
- **THEN** Nasir's row shows IVGA Vastgoed and Whitebird

#### Scenario: The last organisation stays
- **GIVEN** Nasir is a client for IVGA Vastgoed only
- **WHEN** Brian opens Organisations
- **THEN** IVGA Vastgoed can't be unticked

#### Scenario: Staff rows have no Organisations
- **GIVEN** Thomas is a member
- **WHEN** Brian opens Thomas's row menu
- **THEN** there is no Organisations item and Change role offers no Client

### Requirement: Staff assign an issue to a client of its organisation
The assignee picker SHALL list clients in a Clients group between Members and Agents, and only the clients of the issue's organisation: the one organisation of the issue's project. With no project, a project without an organisation or issues of more than one organisation, the Clients group MUST be absent. Members MUST NOT list clients.

#### Scenario: An issue in an organisation's project
- **GIVEN** Nasir is a client for IVGA Vastgoed and Ralph is a client for Whitebird
- **WHEN** Brian opens the assignee picker of an issue in an IVGA Vastgoed project
- **THEN** Clients lists Nasir and not Ralph

#### Scenario: Brian assigns a client wait to the client
- **GIVEN** the issue "Send the 2025 rent roll" is in an IVGA Vastgoed project
- **WHEN** Brian picks Nasir under Clients
- **THEN** Nasir is the assignee of the issue

#### Scenario: An issue without a project
- **GIVEN** an issue has no project
- **WHEN** Brian opens its assignee picker
- **THEN** no Clients group shows

#### Scenario: Several issues of two organisations
- **GIVEN** Brian selected an IVGA Vastgoed issue and a Whitebird issue
- **WHEN** he opens the batch assignee picker
- **THEN** no Clients group shows

#### Scenario: Clients are no members
- **GIVEN** Nasir is a client
- **WHEN** Brian opens the assignee picker of any issue
- **THEN** Members doesn't list Nasir
