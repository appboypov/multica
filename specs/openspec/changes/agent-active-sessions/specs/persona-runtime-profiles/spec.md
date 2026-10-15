## ADDED Requirements

### Requirement: A runtime profile for each persona agent

The persona sync SHALL give Skuddy, Mitchel, Pieter, Pit, Edwin, Anouk, Joeri, Thomas, Roos, Guus, Sarkout and Hermila each a runtime profile whose command is the agent's alias and whose runtime type is the agent's kind, omp today, and SHALL put each of them on its profile. Tika MUST NOT get a profile, and Herdr, Astra, Fable and Haiku SHALL keep their runtimes.

#### Scenario: Mitchel's profile

- **WHEN** the sync runs
- **THEN** a runtime profile with command `mitchel` and runtime type omp exists
- **AND** the agent Mitchel runs on it

#### Scenario: A missing profile

- **GIVEN** the profile `guus` was deleted
- **WHEN** the sync runs
- **THEN** the profile `guus` exists again and Guus runs on it

#### Scenario: The helper agents

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
