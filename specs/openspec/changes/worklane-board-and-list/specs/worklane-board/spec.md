## ADDED Requirements

### Requirement: Board shows every issue of the worklane in a column per workspace status
A worklane's Board SHALL show every issue of the worklane, in a release or not, in one column per status the workspace has, in the workspace's order, Blocked, Cancelled and the workspace's own statuses included, each column with its count; it SHALL keep the column options of Multica's Issues board, with Cancelled among the hidden columns until Brian shows it.

#### Scenario: Fan accounts and push on Board
- **GIVEN** Fan accounts and push holds REBL-21 (Todo, in v1.0.0), REBL-24 (In Progress, in v1.0.1), REBL-27 (Todo, in no release) and REBL-76 (Blocked, in v1.0.0)
- **WHEN** Brian opens its Board
- **THEN** it shows the columns Backlog, Todo, In Progress, In Review, Done and Blocked with Cancelled among the hidden columns
- **AND** Todo shows 2 with REBL-21 and REBL-27, In Progress 1 with REBL-24, and Blocked 1 with REBL-76

#### Scenario: A status of the workspace's own
- **GIVEN** the workspace has its own status Ready for test after In Review
- **WHEN** Brian opens the Board of Fan accounts and push
- **THEN** a column Ready for test follows In Review

#### Scenario: Only this worklane
- **GIVEN** REBL-26 of Agenda and publication is Todo in v1.0.0
- **WHEN** Brian opens the Board of Fan accounts and push
- **THEN** REBL-26 shows in no column

#### Scenario: A worklane without issues
- **GIVEN** Agenda and publication holds no issues
- **WHEN** Brian opens its Board
- **THEN** each column shows "No issues"

### Requirement: A card shows Multica's card with the type icon and the release chip
Each card on a worklane's Board SHALL show what a card on Multica's Issues board shows with its display options, and the issue's type icon and its release chip with the release's version and state.

#### Scenario: REBL-21's card
- **GIVEN** REBL-21 is a story in v1.0.0 First production release, the next release, assigned to Brian with priority High
- **WHEN** Brian opens the Board of Fan accounts and push
- **THEN** REBL-21's card shows the Story icon, REBL-21, its title, Brian's avatar, High and the chip v1.0.0 as the next release

#### Scenario: An issue in no release
- **GIVEN** REBL-27 is in no release
- **WHEN** Brian opens the Board of Fan accounts and push
- **THEN** REBL-27's card shows no release chip

### Requirement: The + of a column opens New issue in that status
Each column of a worklane's Board SHALL have a + that opens New issue with that status, the worklane and the next release of its repository filled in.

#### Scenario: + on In Review
- **GIVEN** v1.0.0 First production release is the next release of appboypov/rebl
- **WHEN** Brian clicks + on In Review on the Board of Fan accounts and push
- **THEN** New issue opens with status In Review, Worklane Fan accounts and push and Release v1.0.0 First production release

#### Scenario: The issue lands in its column
- **GIVEN** New issue opened from + on In Review on the Board of Fan accounts and push
- **WHEN** Brian creates the story "A fan resets their password"
- **THEN** it shows in the In Review column of that Board, in v1.0.0

### Requirement: Dragging a card to another column changes the issue's status
Dropping a card in another column of a worklane's Board SHALL give the issue that column's status and show the toast "{ID} moved to {status}"; when the change fails the card SHALL go back to its column with "Failed to move issue".

#### Scenario: REBL-76 to In Review
- **GIVEN** REBL-76 is In Progress on the Board of Fan accounts and push
- **WHEN** Brian drags its card to In Review
- **THEN** REBL-76 is In Review and the toast says "REBL-76 moved to In Review"

#### Scenario: The change is refused
- **GIVEN** the server refuses REBL-76's status change
- **WHEN** Brian drags its card from In Progress to In Review
- **THEN** the card is back in In Progress and the toast says "Failed to move issue"

### Requirement: An issue that reaches Done gets its done date
An issue SHALL record the moment it reached Done when its status becomes Done and MUST drop it when its status leaves Done.

#### Scenario: Dragged to Done
- **GIVEN** REBL-76 is In Review
- **WHEN** Brian drags its card to Done at 10:05 on 14 October 2026
- **THEN** REBL-76's done date is 10:05 on 14 October 2026

#### Scenario: Back out of Done
- **GIVEN** REBL-76 reached Done at 10:05 on 14 October 2026
- **WHEN** Brian drags its card to In Review
- **THEN** REBL-76 has no done date

### Requirement: A status changed on Board shows wherever the issue shows
A status changed on a worklane's Board SHALL show on that worklane's Timeline and List, on the issue's page, and in the progress of its release.

#### Scenario: REBL-21 to Done
- **GIVEN** v1.0.0 First production release shows "0 of 1 done" over REBL-21 on the Timeline of Fan accounts and push
- **WHEN** Brian drags REBL-21's card to Done on its Board
- **THEN** the Timeline and List show REBL-21 Done and v1.0.0 with "1 of 1 done"
- **AND** REBL-21's page shows Done
