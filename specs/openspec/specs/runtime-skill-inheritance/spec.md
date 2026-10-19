# Runtime skill inheritance

## Purpose

Let an agent inherit its runtime's listed skills, or use its assigned skills with only the runtime skills picked one by one.

## Requirements

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

### Requirement: Runtime skills are inherited by default
The system SHALL treat every existing agent and every new agent as inheriting its runtime's skills until someone turns the section switch off.

#### Scenario: Existing agent inherits after the change ships
- **GIVEN** an agent that existed before this change
- **WHEN** Brian opens its Skills tab
- **THEN** the section switch is on
- **AND** its runs get the runtime's skills as before, minus the skills turned off one by one

#### Scenario: New agent inherits
- **WHEN** Brian creates a Claude, Codex or omp agent and opens its Skills tab
- **THEN** the section switch is on

### Requirement: Later runtime skills follow the section switch
The system SHALL keep a runtime skill that appears on the runtime machine after the section switch was set in a run while the section is on and out of a run while it is off, without anyone changing a setting.

#### Scenario: New skill stays out while the section is off
- **GIVEN** an agent whose section switch is off
- **WHEN** a new skill is installed in the runtime's skill folder and a task of that agent runs
- **THEN** the run does not have the new skill

#### Scenario: New skill comes in while the section is on
- **GIVEN** an agent whose section switch is on
- **WHEN** a new skill is installed in the runtime's skill folder and a task of that agent runs
- **THEN** the run has the new skill

### Requirement: omp agents get per-skill switches
The system SHALL show a switch on every inherited skill row of an omp agent and SHALL keep a skill turned off there out of that agent's runs, as it does for Claude and Codex agents.

#### Scenario: omp rows show switches
- **GIVEN** an omp agent on a local runtime with skills listed under "Inherited from runtime"
- **WHEN** Brian opens its Skills tab
- **THEN** every listed skill row shows a switch, and the section shows its switch

#### Scenario: One skill turned off on an omp agent
- **GIVEN** an omp agent whose section switch is on
- **WHEN** Brian turns off skill "A" and a task of that agent runs
- **THEN** the run does not have "A"
- **AND** the run has every other listed skill

### Requirement: Switches show only where the run keeps a skill out
The system MUST show the section switch and the per-skill switches only for agents on local Claude, Codex and omp runtimes, and MUST reject a change to either setting for any other agent.

#### Scenario: Other provider shows no switches
- **GIVEN** an agent on a provider other than Claude, Codex or omp, with skills listed under "Inherited from runtime"
- **WHEN** Brian opens its Skills tab
- **THEN** the section shows no switch and no row shows a switch

#### Scenario: Section change for another provider is rejected
- **GIVEN** an agent on a provider other than Claude, Codex or omp
- **WHEN** a client asks the server to turn its section switch off
- **THEN** the server rejects the request
- **AND** the agent's section setting stays as it was

### Requirement: Assigned skills stay in
The system SHALL give a run every skill assigned to the agent, also when a runtime skill with the same name is turned off one by one or by the section switch.

#### Scenario: Assigned skill while the section is off
- **GIVEN** an agent with assigned skill "A" and a runtime skill also named "A"
- **WHEN** Brian turns the section switch off and a task of that agent runs
- **THEN** the run has the assigned skill "A"

#### Scenario: Assigned skill while its runtime namesake is turned off
- **GIVEN** an agent with assigned skill "A" and a runtime skill also named "A"
- **WHEN** Brian turns off the runtime skill "A" and a task of that agent runs
- **THEN** the run has the assigned skill "A"

### Requirement: A run never gets turned-off skills
The system MUST NOT start an agent's run with a runtime skill that is turned off: when the daemon cannot list the runtime's skills while the section switch is off, the task fails before the agent starts.

#### Scenario: Skill listing fails while the section is off
- **GIVEN** an agent whose section switch is off
- **WHEN** a task of that agent starts and the daemon cannot list the runtime's skills
- **THEN** the task fails before the agent starts
- **AND** no run of that task has any runtime skill

### Requirement: Only someone who can edit the agent changes the switches
The system MUST let only a person who can manage the agent change its section switch, and MUST reject the change from anyone else.

#### Scenario: Person who cannot manage the agent
- **GIVEN** a workspace member who cannot manage an omp agent
- **WHEN** that member opens the agent's Skills tab
- **THEN** the section shows no switch
- **AND** a request from that member to change the section setting is rejected and the setting stays as it was

### Requirement: Other open clients follow the change
The system SHALL show a changed section switch in every other open web and desktop client that shows the agent, without a reload.

#### Scenario: Second window sees the change
- **GIVEN** the same agent's Skills tab open in two windows
- **WHEN** Brian turns the section switch off in one window
- **THEN** the other window shows the section switch off and its row switches off

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

### Requirement: A missing section setting counts as on
The system SHALL treat an agent or a task claim that carries no section setting, such as one from an older server, as inheriting the runtime's skills.

#### Scenario: Claim from an older server
- **GIVEN** a daemon that receives a task claim without the section setting
- **WHEN** the task's run starts
- **THEN** the run has the runtime's skills minus the skills turned off one by one

#### Scenario: Agent from an older server
- **GIVEN** a web or desktop client that loads an agent without the section setting
- **WHEN** Brian opens its Skills tab
- **THEN** the section switch shows on

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
