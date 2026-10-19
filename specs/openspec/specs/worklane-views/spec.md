# worklane-views Specification

## Purpose
Lets Brian switch a worklane between Timeline, Board and List and filter all three by type.

## Requirements

### Requirement: Brian switches a worklane between Timeline, Board and List
A worklane's header SHALL offer the views Timeline, Board and List, open on Timeline, and show the picked view of the same worklane.

#### Scenario: From Timeline to Board and List
- **GIVEN** Brian opened Fan accounts and push of appboypov/rebl from Rebl's page and it shows its Timeline
- **WHEN** he picks Board and then List
- **THEN** Board shows the issues of Fan accounts and push by status
- **AND** List shows them by release

### Requirement: The Type filter applies to Timeline, Board and List
A worklane's header SHALL offer a Type filter with the eleven types and No type; while types are picked, Timeline, Board and List SHALL show only the worklane's issues of those types, and switching views MUST keep the picked types.

#### Scenario: Only bugs
- **GIVEN** Fan accounts and push holds the story REBL-21 and the bug REBL-24
- **WHEN** Brian picks Bug in the Type filter on Board
- **THEN** Board shows REBL-24 and not REBL-21
- **AND** after he switches to List and Timeline, both show REBL-24 and not REBL-21 with Bug still picked

#### Scenario: Issues without a type
- **GIVEN** Fan accounts and push holds REBL-27, which has no type
- **WHEN** Brian picks No type in the Type filter
- **THEN** each view shows REBL-27 and none of the worklane's typed issues
