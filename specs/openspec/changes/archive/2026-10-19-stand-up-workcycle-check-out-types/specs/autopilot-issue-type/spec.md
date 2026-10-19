## ADDED Requirements

### Requirement: An autopilot makes its issues with a type
An autopilot SHALL take an optional issue type, one of the eleven, and SHALL refuse any other value. Each issue its `create_issue` run makes SHALL have that type, and its description SHALL hold that type's template Markdown for the run to fill in. Without an issue type the issue SHALL have no type.

#### Scenario: A stand-up autopilot
- **GIVEN** the We Work autopilot has the issue type standup and the title template "Stand-up We Work {{date}}"
- **WHEN** it runs on 20 October 2026
- **THEN** it makes "Stand-up We Work 2026-10-20" as a stand-up whose description holds the stand-up template

#### Scenario: An unknown issue type
- **WHEN** an agent runs `multica autopilot create` with `--issue-type task`
- **THEN** it fails and makes no autopilot

### Requirement: Stand-up autopilots from before have their type
Every autopilot whose title template starts with "Stand-up " before issue types on autopilots existed SHALL have the issue type standup.

#### Scenario: An earlier stand-up autopilot
- **GIVEN** the Spar Kaart autopilot makes "Stand-up Spar Kaart {{date}}" with no issue type
- **WHEN** the release migrates the database
- **THEN** the autopilot has the issue type standup
