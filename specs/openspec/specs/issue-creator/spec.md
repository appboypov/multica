# issue-creator Specification

## Purpose

Credits issues created by an agent to the member the run works for while retaining the filing agent's activity attribution.

## Requirements

### Requirement: An issue an agent makes for a member has that member as creator
An issue an agent creates in a run or a quick create SHALL have the member the run works for as its creator. The issue's origin SHALL still name the run, so its activity shows which agent filed it. An agent run that works for no member SHALL stay the creator.

#### Scenario: Quick create
- **GIVEN** Skuddy can file issues for Brian
- **WHEN** Brian quick-creates "Fix the login timeout" and Skuddy files it as WORK-430
- **THEN** WORK-430's creator is Brian and its activity shows that Skuddy created it

#### Scenario: Agent in a run
- **GIVEN** Joeri works in a run Brian started
- **WHEN** Joeri runs `multica issue create` for WORK-431
- **THEN** WORK-431's creator is Brian

### Requirement: Issues made before get their creator
Every existing issue an agent created in a run or a quick create SHALL get the member that run works for as its creator.

#### Scenario: Existing quick create
- **GIVEN** WORK-300 was quick-created by Brian and shows Skuddy as creator
- **WHEN** the migration runs
- **THEN** WORK-300's creator is Brian
