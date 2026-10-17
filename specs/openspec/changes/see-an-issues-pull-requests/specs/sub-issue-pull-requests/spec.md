## ADDED Requirements

### Requirement: An issue lists the pull requests of its sub-issues
The Pull requests section of an issue on web and desktop SHALL show its own pull requests and then the pull requests of every issue below it at any depth. Each sub-issue pull request SHALL show the key of the sub-issue it belongs to. A pull request linked to the issue itself MUST show once as its own.

#### Scenario: A journey and its steps
- **GIVEN** journey issue WORK-900 has the step sub-issues WORK-901 with PR 40 and WORK-902 with PR 41
- **WHEN** Brian opens WORK-900
- **THEN** its Pull requests section shows PR 40 with WORK-901 and PR 41 with WORK-902

#### Scenario: A pull request two levels down
- **GIVEN** epic WORK-800 has story WORK-810 and WORK-810 has sub-issue WORK-811 with PR 52
- **WHEN** Brian opens WORK-800
- **THEN** its Pull requests section shows PR 52 with WORK-811

#### Scenario: The parent's own pull request comes first and once
- **GIVEN** PR 40 is linked to journey issue WORK-900 and to its step WORK-901
- **WHEN** Brian opens WORK-900
- **THEN** PR 40 shows once as WORK-900's own pull request

#### Scenario: One pull request on two sub-issues
- **GIVEN** PR 26 is linked to the sub-issues WORK-341 and WORK-342 of WORK-338
- **WHEN** Brian opens WORK-338
- **THEN** PR 26 shows once with WORK-341 and WORK-342

#### Scenario: An issue without sub-issues
- **GIVEN** WORK-343 has no sub-issues and PR 31 is linked to it
- **WHEN** Brian opens WORK-343
- **THEN** its Pull requests section shows only PR 31

### Requirement: A sub-issue's pull request is not the parent's link
A pull request that shows on an issue only through a sub-issue MUST NOT count toward the issue's pull request auto-complete, and the issue MUST NOT offer Remove for it.

#### Scenario: Merging a step's pull request
- **GIVEN** journey issue WORK-900 has no pull request of its own and its step WORK-901 has PR 40 with "Closes WORK-901"
- **WHEN** PR 40 merges
- **THEN** WORK-901 completes as today
- **AND** WORK-900 keeps its status

#### Scenario: No Remove on the parent
- **GIVEN** WORK-900 shows PR 40 with WORK-901
- **WHEN** Brian opens the menu of PR 40 on WORK-900
- **THEN** it offers no Remove

### Requirement: The CLI lists the pull requests of sub-issues
`multica issue pull-requests` SHALL list an issue's own pull requests and the pull requests of every issue below it, each sub-issue pull request with the key of its sub-issue, in table and JSON output.

#### Scenario: Listing a journey from the CLI
- **GIVEN** journey issue WORK-900 has step WORK-901 with PR 40
- **WHEN** an agent runs `multica issue pull-requests WORK-900 --output json`
- **THEN** the output holds PR 40 with WORK-901 as its issue
