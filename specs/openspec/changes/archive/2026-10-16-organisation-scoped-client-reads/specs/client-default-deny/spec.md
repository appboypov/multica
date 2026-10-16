## MODIFIED Requirements

### Requirement: A client is refused outside the session list
Multica SHALL refuse a client's request on every route before the route's own handler runs, except the routes of the session list, and a route MUST serve a client only when it is on that list.

| Session list | What it lets a client do |
|---|---|
| Authentication | Log in, refresh the session and log out |
| Own invitation | Read, accept or decline an invitation addressed to them |
| Current membership | Read their own role and organisations, as `client-membership` says |
| Own waits | List their own open waits and read one by its ID, as `client-reads` says |
| Planned work | List the planned work of their organisations, as `client-reads` says |

#### Scenario: A write route
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar creates an issue
- **THEN** the request is refused and no issue is created

#### Scenario: A read route
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar lists the workspace's issues, projects, releases or members through their workspace routes
- **THEN** each request is refused and returns no workspace data

#### Scenario: A route added later
- **GIVEN** Rizgar is a client of Rebl Records
- **AND** a workspace route exists that is not on the session list and carries no client rule of its own
- **WHEN** Rizgar calls that route
- **THEN** the request is refused before the route's handler runs

#### Scenario: The session list stays open
- **GIVEN** Rizgar is a client of Rebl Records
- **WHEN** Rizgar refreshes his session, reads his current membership, lists his waits, lists planned work and logs out
- **THEN** each request succeeds

### Requirement: A client is refused on direct resource IDs
Multica MUST refuse a client's request that names a workspace resource by its ID or key, such as an issue, a comment, a project or an attachment, outside the session list.

#### Scenario: Opening an issue by its key
- **GIVEN** Rizgar is a client of Rebl Records and WORK-12 is his own wait in a Rebl Records project, on a release
- **WHEN** Rizgar requests WORK-12 by its key or its ID through the workspace's issue route
- **THEN** the request is refused and returns none of WORK-12's fields

#### Scenario: Downloading an attachment by its ID
- **GIVEN** Rizgar is a client of Rebl Records and an attachment exists on WORK-12
- **WHEN** Rizgar requests the attachment's download by its ID
- **THEN** the request is refused and returns no file
