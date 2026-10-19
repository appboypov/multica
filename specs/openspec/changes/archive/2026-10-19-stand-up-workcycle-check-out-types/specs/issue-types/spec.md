## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Stand-ups, workcycles and check-outs from before have their type
Every issue whose title starts with "Stand-up ", "Workcycle " or "Check-out " before these types existed SHALL have the type Stand-up, Workcycle or Check-out.

#### Scenario: An earlier check-out
- **GIVEN** WORK-1843 "Check-out We Work workcycle 20261019-001" had no type
- **WHEN** Brian opens Issues after the release
- **THEN** WORK-1843 shows the Check-out icon
