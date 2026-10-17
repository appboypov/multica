# office-agents Specification

## Purpose
How the agents of the offices in `~/Repos/Agents` take part in the Multica workspace De App Specialist: as an agent with a runtime, or as a member.

## Requirements

### Requirement: Paul works in Multica on this Mac

Paul SHALL be a Multica agent of the workspace De App Specialist that runs on this Mac's runtime, where the day's traces are.

#### Scenario: Paul's agent

- **GIVEN** the Multica workspace De App Specialist
- **WHEN** `multica agent list` runs
- **THEN** Paul is an active agent with his persona as its instructions
- **AND** his runtime is this Mac's omp runtime

#### Scenario: Brian gives feedback on a report

- **GIVEN** an improvement report task
- **WHEN** Brian names Paul in a comment on it
- **THEN** Paul runs on that task and answers there

### Requirement: Tika is a Multica member

Tika SHALL be a member of the Multica workspace "De App Specialist", not an agent, so that Multica never starts a run of Tika; she SHALL work with Multica from her session in `#tika` `#chat`.

#### Scenario: One Tika

- **GIVEN** Tika is in the workspace "De App Specialist"
- **WHEN** Multica lists the workspace's members and agents
- **THEN** Tika is a member and not an agent
- **AND** assigning her a task starts no Multica run
