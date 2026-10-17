# madspec-sync-workers Specification

## Purpose
Keep each agent and worker of the agents repo a managed Multica agent through the manual sync, without touching squads or unmanaged agents.

## Requirements

### Requirement: Agents and workers sync by hand
The manual sync SHALL create one managed local omp agent per folder of the agents repo with an `agent.toml`, using its `AGENTS.md` as instructions and its model list as the primary model and ordered fallbacks.

#### Scenario: A worker is synced
- **GIVEN** a worker folder with its `agent.toml` and `AGENTS.md`
- **WHEN** the sync runs
- **THEN** its managed agent has that name, instructions, model and effort

### Requirement: Preview preserves user data
Dry run SHALL report proposed creates and changes without writing API or overlay state. Every sync SHALL leave squads and unmanaged agents unchanged, except for an explicit unambiguous `--adopt NAME`.

#### Scenario: Preview
- **WHEN** the sync runs with --dry-run
- **THEN** proposed writes are printed and no state is written

#### Scenario: An unmanaged name conflicts
- **GIVEN** an unmanaged agent with a source name
- **WHEN** the sync runs
- **THEN** it reports the conflict without changing the agent

#### Scenario: Adopt Lukas by name
- **GIVEN** one unmanaged agent named Lukas and his folder
- **WHEN** the sync runs with --adopt Lukas
- **THEN** that agent becomes managed, keeps other environment values and receives his folder's configuration
