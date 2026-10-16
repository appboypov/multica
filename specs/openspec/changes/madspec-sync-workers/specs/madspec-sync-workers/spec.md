## ADDED Requirements

### Requirement: Office workers and personas sync by hand
The manual sync SHALL read office personas with both roster roots and create one managed local omp agent per roster worker, using its body as instructions and its model list as the primary model and ordered fallbacks.

#### Scenario: A worker is synced
- **GIVEN** a roster worker file in its lead office
- **WHEN** the sync runs
- **THEN** its managed agent has that name, instructions, model, effort and fallback chain

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
- **GIVEN** one unmanaged agent named Lukas and his office persona
- **WHEN** the sync runs with --adopt Lukas
- **THEN** that agent becomes managed, keeps other environment values and receives his office configuration
