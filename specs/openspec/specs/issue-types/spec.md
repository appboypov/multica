# issue-types Specification

## Purpose
Lets Brian tell an issue's type by its icon, see "No type" on an issue without one, and set that type once.

## Requirements

### Requirement: An issue shows its type icon wherever it shows
An issue SHALL show its type's icon next to its ID wherever it shows, and an issue without a type SHALL show the No type icon.

#### Scenario: On Issues and the Timeline
- **GIVEN** REBL-21 is a story and REBL-25 a bug
- **WHEN** Brian opens Issues and the Timeline of Fan accounts and push
- **THEN** REBL-21 shows the Story icon and REBL-25 the Bug icon in both

#### Scenario: An issue from before
- **GIVEN** MUL-12 was in Multica before issue types existed
- **WHEN** Brian opens Issues
- **THEN** MUL-12 shows the No type icon

### Requirement: An issue without a type gets its type once on its page
An issue without a type SHALL show Type "No type" on its page and offer the eleven types there once; setting it SHALL keep the description as it is. An issue's type MUST NOT change once set.

#### Scenario: Setting the type
- **GIVEN** MUL-12 has no type and a description written before issue types existed
- **WHEN** Brian sets its Type to Bug on its page
- **THEN** MUL-12 shows the Bug icon and its description is unchanged

#### Scenario: A type is fixed
- **GIVEN** REBL-21 is a story
- **WHEN** Brian opens REBL-21's page
- **THEN** Type shows Story and offers no other type

### Requirement: Stand-ups, workcycles and check-outs from before have their type
Every issue whose title starts with "Stand-up ", "Workcycle " or "Check-out " before these types existed SHALL have the type Stand-up, Workcycle or Check-out.

#### Scenario: An earlier check-out
- **GIVEN** WORK-1843 "Check-out We Work workcycle 20261019-001" had no type
- **WHEN** Brian opens Issues after the release
- **THEN** WORK-1843 shows the Check-out icon
