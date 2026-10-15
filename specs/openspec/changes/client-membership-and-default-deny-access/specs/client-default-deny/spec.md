## ADDED Requirements

### Requirement: A client is refused outside the session list
Multica SHALL refuse a client's request on every route before the route's own handler runs, except the routes of the session list, and a route MUST serve a client only when it is on that list.

| Session list | What it lets a client do |
|---|---|
| Authentication | Log in, refresh the session and log out |
| Own invitation | Read, accept or decline an invitation addressed to them |
| Current membership | Read their own role and organisation, as `client-membership` says |

#### Scenario: A write route
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar creates an issue
- **THEN** the request is refused and no issue is created

#### Scenario: A read route
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar lists the workspace's issues, projects or members
- **THEN** each request is refused and returns no workspace data

#### Scenario: A route added later
- **GIVEN** Rizgar is a client of Rebl Records
- **AND** a workspace route exists that is not on the session list and carries no client rule of its own
- **WHEN** Rizgar calls that route
- **THEN** the request is refused before the route's handler runs

#### Scenario: The session list stays open
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar refreshes his session, reads his current membership and logs out
- **THEN** each request succeeds

### Requirement: A client is refused on direct resource IDs
Multica MUST refuse a client's request that names a workspace resource by its ID or key, such as an issue, a comment, a project or an attachment, outside the session list.

#### Scenario: Opening an issue by its key
- **GIVEN** Rizgar is a client of Rebl Records and WORK-12 is in a Rebl Records project
- **WHEN** Rizgar requests WORK-12 by its key or its ID
- **THEN** the request is refused and returns none of WORK-12's fields

#### Scenario: Downloading an attachment by its ID
- **GIVEN** Rizgar is a client of Rebl Records and an attachment exists on WORK-12
- **WHEN** Rizgar requests the attachment's download by its ID
- **THEN** the request is refused and returns no file

### Requirement: A client is refused on user-scoped entry points
Multica MUST refuse a client on every entry point outside a workspace's routes that can return workspace data, such as global search, export and the inbox.

#### Scenario: Global search
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar searches for "Rebl"
- **THEN** the request is refused and returns no results

#### Scenario: Export
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar asks for an export of the workspace
- **THEN** the request is refused and returns no export

### Requirement: A client's credentials are refused the same way on every surface
Multica SHALL apply the client refusal to every credential of the client, such as a browser session and a personal access token used by the CLI.

#### Scenario: The CLI with a client's token
- **GIVEN** Rizgar is a client of Rebl Records with a personal access token
- **WHEN** Rizgar runs `multica issue list` with that token
- **THEN** the request is refused and lists no issues

### Requirement: A client starts no agent task
A client's request MUST NOT start an agent task.

#### Scenario: Assigning an issue to an agent
- **GIVEN** Rizgar is a client of Rebl Records and the agent Joeri exists
- **WHEN** Rizgar assigns WORK-12 to Joeri or mentions Joeri in a comment on WORK-12
- **THEN** the request is refused and no agent task starts

### Requirement: A client gets no workspace live updates
Multica MUST refuse a client's workspace live-update subscription, MUST stop the workspace live updates of a connected session whose member becomes a client, and MUST NOT send a client workspace event payloads through any other live channel.

#### Scenario: A client subscribes
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar opens the workspace's live-update connection
- **THEN** the subscription is refused

#### Scenario: A connected member becomes a client
- **GIVEN** Sam is a member with an open live-update connection to the workspace
- **WHEN** Brian changes Sam's role to `client` for Rebl Records
- **AND** Brian then updates WORK-12
- **THEN** Sam's connection receives no event about WORK-12 or any later workspace event

#### Scenario: No workspace event on a personal channel
- **GIVEN** Rizgar is a client of Rebl Records and WORK-12 is assigned to him
- **WHEN** Brian comments on WORK-12
- **THEN** no live channel of Rizgar receives the comment or any other field of WORK-12

### Requirement: Owners, admins and members keep their access
Multica SHALL give owners, admins and members the same routes, data and live updates they had before the client role existed.

#### Scenario: Staff work goes on
- **GIVEN** Brian is an admin and Sam is a member
- **WHEN** they list issues, open WORK-12, search, and subscribe to workspace live updates
- **THEN** each request succeeds as before

#### Scenario: A member list that holds a client
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Brian opens the workspace's members in the web, desktop or mobile app
- **THEN** the list loads and lists Rizgar
