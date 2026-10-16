# client-reads Specification

## Purpose
Clients read their own open waits and the released work of their current organisations through narrow, visibility-filtered responses.

## Requirements

### Requirement: A client lists their own open waits
Multica SHALL list for a client every own wait: an issue of type `chore` with the label `Client wait`, assigned to the client, in a project whose organisation is one of the client's organisations, and whose status is not in the done or closed category, with or without a release.

#### Scenario: Waits in two organisations
- **GIVEN** Nasir is a client of IVGA Vastgoed and Whitebird
- **AND** "Send the rent roll" is an open `Client wait` chore assigned to Nasir in an IVGA Vastgoed project, on no release
- **AND** "Approve the new logo" is an open `Client wait` chore assigned to Nasir in a Whitebird project
- **WHEN** Nasir lists his waits
- **THEN** both waits are listed, each with its organisation

#### Scenario: A finished wait
- **GIVEN** Nasir is a client of IVGA Vastgoed and his `Client wait` chore there is done
- **WHEN** Nasir lists his waits
- **THEN** the done wait is not listed

#### Scenario: Another contact's wait
- **GIVEN** Nasir and Rizgar are clients of IVGA Vastgoed and a `Client wait` chore there is assigned to Rizgar
- **WHEN** Nasir lists his waits
- **THEN** Rizgar's wait is not listed

#### Scenario: A wait in an organisation of someone else
- **GIVEN** Nasir is a client of IVGA Vastgoed only
- **AND** a `Client wait` chore assigned to Nasir is in a project of Rebl Records
- **WHEN** Nasir lists his waits
- **THEN** that wait is not listed

#### Scenario: An issue on his name that is not a wait
- **GIVEN** Nasir is a client of IVGA Vastgoed
- **AND** a story without the `Client wait` label in an IVGA Vastgoed project is assigned to Nasir
- **WHEN** Nasir lists his waits
- **THEN** the story is not listed

#### Scenario: A reassigned wait
- **GIVEN** Nasir's open `Client wait` chore in IVGA Vastgoed is reassigned to Rizgar
- **WHEN** Nasir lists his waits
- **THEN** the wait is not listed

#### Scenario: A removed organisation
- **GIVEN** Nasir is a client of IVGA Vastgoed and Whitebird with an open wait in each
- **WHEN** Brian removes Whitebird from Nasir's organisations and Nasir lists his waits
- **THEN** only the IVGA Vastgoed wait is listed

### Requirement: A client reads one own wait
Multica SHALL return an own wait to its client by the issue's ID, and MUST answer a request for any other issue ID, including an existing issue, as it answers an ID that does not exist, with none of the issue's fields.

#### Scenario: Reading an own wait
- **GIVEN** "Send the rent roll" is an open own wait of Nasir
- **WHEN** Nasir reads it by its ID
- **THEN** the response holds its title, description, due date and organisation

#### Scenario: Reading another contact's wait
- **GIVEN** a `Client wait` chore in IVGA Vastgoed is assigned to Rizgar
- **WHEN** Nasir, a client of IVGA Vastgoed, reads it by its ID
- **THEN** the response is the same as for an ID that does not exist and holds no field of the wait

#### Scenario: Reading an issue of another organisation
- **GIVEN** WORK-12 is in a Rebl Records project and Nasir is no client of Rebl Records
- **WHEN** Nasir reads WORK-12 by its ID
- **THEN** the response is the same as for an ID that does not exist and holds no field of WORK-12

### Requirement: A client lists the planned work of their organisations
Multica SHALL list for a client every issue on a release in a project whose organisation is one of the client's organisations, whatever its type, status or assignee, and MUST NOT list an issue without a release.

#### Scenario: Released work of two organisations
- **GIVEN** Nasir is a client of IVGA Vastgoed and Whitebird
- **AND** each has a project with an issue on a release
- **WHEN** Nasir lists planned work
- **THEN** both issues are listed, each with its organisation

#### Scenario: Unreleased work
- **GIVEN** a story in an IVGA Vastgoed project is on no release
- **WHEN** Nasir lists planned work
- **THEN** the story is not listed

#### Scenario: A release is removed
- **GIVEN** a story on a release in an IVGA Vastgoed project is listed as planned work for Nasir
- **WHEN** Brian takes the story off its release and Nasir lists planned work again
- **THEN** the story is not listed

#### Scenario: Another organisation's released work
- **GIVEN** Rebl Records has released work and Nasir is no client of Rebl Records
- **WHEN** Nasir lists planned work
- **THEN** no Rebl Records issue is listed

#### Scenario: A project without an organisation
- **GIVEN** a project has no organisation and holds an issue on a release
- **WHEN** Nasir lists planned work
- **THEN** that issue is not listed

### Requirement: Client rows hold only the approved fields
A client read SHALL return each row with only the fields of its kind in the table below, and MUST NOT return any other field of the issue, its project, release, organisation or assignee.

| Kind | Fields |
|---|---|
| Own wait | ID, title, description, due date, organisation name |
| Planned work | ID, title, status, release version, whether the release has shipped, project name, organisation name |

#### Scenario: Private fields of planned work
- **GIVEN** a released story in IVGA Vastgoed has a description, a due date, Points, comments, an attachment and an agent run
- **WHEN** Nasir lists planned work
- **THEN** the story's row holds its ID, title, status, release version, shipped state, project name and organisation name, and nothing else

#### Scenario: Another contact's wait on a release
- **GIVEN** Rizgar's `Client wait` chore in IVGA Vastgoed is on a release
- **WHEN** Nasir lists planned work
- **THEN** the wait is listed with the planned-work fields only, without its description or due date

#### Scenario: Private fields of an own wait
- **GIVEN** Nasir's own wait has Points, comments, an attachment, custom properties and source context
- **WHEN** Nasir lists his waits and reads the wait by its ID
- **THEN** each response holds only its ID, title, description, due date and organisation name

### Requirement: Caller input cannot widen a client read
Multica MUST derive the client, the organisations and the assignee of a client read from the client's session and current membership, and a client's organisation, project, assignee, release or issue identifier in the request MUST NOT add a row or a field to the response.

#### Scenario: Naming another organisation
- **GIVEN** Nasir is a client of IVGA Vastgoed only
- **WHEN** Nasir lists planned work and his waits with Rebl Records' organisation ID, a Rebl Records project ID or Rizgar's user ID in the request
- **THEN** no Rebl Records issue and no wait of Rizgar is listed

### Requirement: Counts and pages hold only visible rows
Multica SHALL apply client visibility before pagination and counts, so that every page holds only rows the client may see and every total counts only those rows.

#### Scenario: Paging through planned work
- **GIVEN** Nasir may see 3 released issues and the workspace has 50 released issues
- **WHEN** Nasir lists planned work two at a time
- **THEN** the total is 3, the first page holds 2 visible rows and the second page holds the last visible row

#### Scenario: Counting waits
- **GIVEN** Nasir has 2 open own waits, 1 done own wait, and Rizgar has 4 open waits in the same organisation
- **WHEN** Nasir lists his waits
- **THEN** the total is 2
