# Persona runtime profiles

## Purpose

Run each agent and worker of the agents repo through its alias while keeping model and thinking choices made in Multica.

## Requirements

### Requirement: A runtime profile for each agent

The persona sync SHALL give every folder of the agents repo with an `agent.toml`, workers included, a runtime profile whose command is the folder's alias and whose runtime type is omp, and SHALL put the folder's Multica agent on it. A folder without an `agent.toml`, such as Tika's, MUST NOT get a profile, and Herdr, Astra, Fable and Haiku SHALL keep their runtimes.

#### Scenario: Mitchel's profile

- **GIVEN** Mitchel's alias is available
- **WHEN** the sync runs
- **THEN** a runtime profile with command `mitchel` and runtime type omp exists
- **AND** the agent Mitchel runs on it

#### Scenario: A missing profile

- **GIVEN** the profile `guus` was deleted
- **WHEN** the sync runs
- **THEN** the profile `guus` exists again and Guus runs on it

#### Scenario: A new agent keeps its Multica edits

- **GIVEN** Multica has no agent Paul and `paul/agent.toml` names his models
- **WHEN** the full sync runs with the profile runtime registered
- **THEN** an active agent Paul runs on his profile, seeded from the first entry of his models
- **AND** later syncs preserve model and thinking edits made in Multica and leave archived agents archived

#### Scenario: The helper agents

- **GIVEN** Herdr, Astra, Fable and Haiku have runtimes and Tika's folder has no `agent.toml`
- **WHEN** the sync runs
- **THEN** Herdr, Astra, Fable and Haiku run on the runtimes they ran on before
- **AND** no profile exists for Tika

### Requirement: A run starts the agent through its alias

A run of a persona agent SHALL start the agent with its alias in the run's work folder, so the run has the agent's office loaded the same way as when Brian starts the alias by hand.

#### Scenario: Mitchel is tagged

- **GIVEN** Mitchel runs on his profile
- **WHEN** Skuddy tags Mitchel on WORK-1057
- **THEN** the run starts `mitchel` in the issue's work folder
- **AND** the run follows `~/Repos/Agents/mitchel/AGENTS.md`
