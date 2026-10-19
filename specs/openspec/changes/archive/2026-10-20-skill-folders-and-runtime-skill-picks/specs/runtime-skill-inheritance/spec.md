## MODIFIED Requirements

### Requirement: Section switch keeps the runtime's skills out
The system SHALL give the "Inherited from runtime" section of an agent's Capabilities > Skills tab one switch that, while off, keeps every runtime skill the section lists out of the agent's runs except the skills turned on one by one, so a run gets the agent's assigned skills and only the picked runtime skills.

#### Scenario: Section off gives a run only the assigned skills
- **GIVEN** an omp agent with assigned skills and runtime skills listed under "Inherited from runtime"
- **WHEN** Brian turns the section switch off and a task of that agent runs
- **THEN** the run has the agent's assigned skills
- **AND** the run has none of the skills the section lists

#### Scenario: Section on gives a run the runtime's skills again
- **GIVEN** an agent whose section switch is off
- **WHEN** Brian turns the section switch on and a task of that agent runs
- **THEN** the run has every skill the section lists, beside the assigned skills

#### Scenario: Section off on a Claude agent
- **GIVEN** a Claude agent with runtime skills listed under "Inherited from runtime"
- **WHEN** Brian turns the section switch off and a task of that agent runs
- **THEN** the run has none of the skills the section lists

### Requirement: Section switch shows only with listed skills
The system SHALL hide the section switch while the section lists no skill that can be turned off, and SHALL keep the stored setting in force while it is hidden.

#### Scenario: Empty section hides the switch
- **GIVEN** an agent whose section switch is off and whose runtime lists no skills
- **WHEN** Brian opens its Skills tab
- **THEN** the section shows no switch

#### Scenario: Stored off holds once a skill appears
- **GIVEN** an agent whose section switch is off and whose runtime listed no skills
- **WHEN** a skill is installed in the runtime's skill folder and Brian refreshes the section
- **THEN** the section switch shows off and the skill's row switch shows off
- **AND** Brian can turn the skill's row switch on

### Requirement: Other open clients follow the change
The system SHALL show a changed section switch in every other open web and desktop client that shows the agent, without a reload.

#### Scenario: Second window sees the change
- **GIVEN** the same agent's Skills tab open in two windows
- **WHEN** Brian turns the section switch off in one window
- **THEN** the other window shows the section switch off and its row switches off

## REMOVED Requirements

### Requirement: Per-skill choices survive the section switch
**Reason**: Brian turns inheritance off to pick a few skills, and each flip of the section switch now sets every skill on or off.
**Migration**: "Each flip of the section switch sets every skill" and "Skills are picked one by one in both section states" replace it.

## ADDED Requirements

### Requirement: Each flip of the section switch sets every skill
The system SHALL turn every runtime skill of the agent on when the section switch is turned on and off when it is turned off, and SHALL clear the per-skill choices made before the flip.

#### Scenario: Turning the section off clears skills turned off before
- **GIVEN** an agent with skill "B" turned off and the section switch on
- **WHEN** Brian turns the section switch off and then on again
- **THEN** every row switch shows on, "B" included
- **AND** a run of that agent has every listed skill

#### Scenario: Turning the section on clears skills picked before
- **GIVEN** an agent whose section switch is off with skill "A" turned on
- **WHEN** Brian turns the section switch on and then off again
- **THEN** every row switch shows off, "A" included

### Requirement: Skills are picked one by one in both section states
The system SHALL keep every runtime skill's row switch changeable in both section states: while the section is on Brian turns single skills off, and while it is off he turns single skills on, and a run follows those picks.

#### Scenario: One skill picked while the section is off
- **GIVEN** an omp agent whose section switch is off, with skills "A" and "B" listed
- **WHEN** Brian turns on skill "A" and a task of that agent runs
- **THEN** the run has "A"
- **AND** the run does not have "B"
