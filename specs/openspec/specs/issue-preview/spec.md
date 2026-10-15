# issue-preview Specification

## Purpose
Lets Brian open an issue's preview from a worklane's views and see and change its status, worklane and release there.

## Requirements

### Requirement: Clicking an issue on Timeline, Board or List opens its preview
Clicking a card on a worklane's Board or an issue on its Timeline or List SHALL open the issue's preview, Multica's issue preview, beside the view.

#### Scenario: From Board
- **WHEN** Brian clicks REBL-21's card on the Board of Fan accounts and push
- **THEN** REBL-21's preview opens beside the Board

#### Scenario: From List and Timeline
- **WHEN** Brian clicks REBL-24's row on the List of Fan accounts and push, and later REBL-24 on its Timeline
- **THEN** each time REBL-24's preview opens

### Requirement: The preview shows status, worklane and release at the top
An issue's preview SHALL show at its top the type icon and type, the ID, the title, a status menu, the worklane as a link to its Timeline, the release chip, and the assignee, followed by the description and "Open issue", wherever the preview opens.

#### Scenario: REBL-21's preview
- **GIVEN** REBL-21 is a Todo story in Fan accounts and push and v1.0.0 First production release, assigned to Brian
- **WHEN** Brian opens its preview
- **THEN** it shows Story, REBL-21, "A fan changes their display name", Todo, Fan accounts and push, the chip v1.0.0 and Brian above its description

#### Scenario: On Issues
- **GIVEN** REBL-21 shows on Multica's Issues page
- **WHEN** Brian clicks it there
- **THEN** its preview shows Fan accounts and push and v1.0.0 at its top

#### Scenario: An issue without a worklane
- **GIVEN** MUL-12 has no worklane and no release
- **WHEN** Brian opens its preview
- **THEN** it shows "No worklane" and no release chip

### Requirement: The preview changes the status and leads to the worklane and the issue's page
Picking a status in the preview's status menu SHALL change the issue's status in the open view; clicking the worklane SHALL open that worklane's Timeline, and "Open issue" SHALL open the issue's page.

#### Scenario: Status from the preview
- **GIVEN** REBL-21's preview is open over the Board of Fan accounts and push
- **WHEN** Brian picks In Progress in its status menu
- **THEN** REBL-21's card moves to the In Progress column

#### Scenario: To the worklane
- **GIVEN** REBL-21's preview is open on Multica's Issues page
- **WHEN** Brian clicks Fan accounts and push in it
- **THEN** the Timeline of Fan accounts and push opens

#### Scenario: Open issue
- **WHEN** Brian clicks "Open issue" in REBL-21's preview
- **THEN** REBL-21's page opens
