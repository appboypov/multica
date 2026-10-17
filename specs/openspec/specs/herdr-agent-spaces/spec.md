# herdr-agent-spaces Specification

## Purpose
Herdr starts each agent in its own space and routes work from the current Multica agents and runtime profiles.

## Requirements

### Requirement: A Herdr space for each agent

Each agent with an office SHALL have its own Herdr space `#<alias>`, whose first tab `#root` is where the agent starts by default. Tika's default tab SHALL be `#chat`.

#### Scenario: Mitchel's space

- **GIVEN** the offices are set up
- **WHEN** Brian lists the Herdr spaces
- **THEN** `#mitchel` is there with the tab `#root`

#### Scenario: Tika's space

- **GIVEN** the offices are set up
- **WHEN** Brian lists the tabs of `#tika`
- **THEN** `#chat` is there and is Tika's default tab

### Requirement: The Herdr agent opens a tagged agent in its space

When the Herdr agent is tagged to start an agent, it SHALL start that agent with its command in the agent's space: in `#root` when no agent runs there, else in a new tab named after the issue key of the issue it was tagged on. It SHALL create the space with its `#root` tab when the space is missing. The session SHALL start in the folder the request names, else in the agent's office.

#### Scenario: Root is free

- **GIVEN** no agent runs in `#mitchel` `#root`
- **WHEN** Brian tags the Herdr agent on WORK-1057 to start Mitchel
- **THEN** `mitchel` runs in `#mitchel` `#root`

#### Scenario: Root is busy

- **GIVEN** a Mitchel session runs in `#mitchel` `#root`
- **WHEN** Brian tags the Herdr agent on WORK-1057 to start Mitchel
- **THEN** a new tab `#work-1057` opens in `#mitchel` with `mitchel` running in it
- **AND** the session in `#root` keeps running

#### Scenario: The space is missing

- **GIVEN** Herdr has no space `#anouk`
- **WHEN** Brian tags the Herdr agent to start Anouk
- **THEN** the space `#anouk` exists with the tab `#root`, and `anouk` runs in it

#### Scenario: No folder named

- **GIVEN** a request that names no folder
- **WHEN** the Herdr agent starts Pieter
- **THEN** Pieter's session works in `~/Repos/Agents/pieter`

### Requirement: The Herdr agent reads the agents from Multica

The Herdr agent SHALL work out the agents, their commands and their spaces from Multica at each request. Its instructions SHALL hold the rules and MUST NOT hold a list of agents.

#### Scenario: A new agent

- **GIVEN** a new agent with an office and a runtime profile in Multica
- **WHEN** Brian tags the Herdr agent to start it
- **THEN** the agent starts in its own space
- **AND** the Herdr agent's instructions were not changed for it

### Requirement: A tag of another agent opens no tab

A tag of an agent other than the Herdr agent in Multica SHALL start a headless run of that agent, and MUST NOT open a Herdr tab.

#### Scenario: Mitchel is tagged

- **GIVEN** no Mitchel session runs in Herdr
- **WHEN** Skuddy tags Mitchel on an issue
- **THEN** Mitchel's run starts in Multica
- **AND** no tab opens in `#mitchel`
