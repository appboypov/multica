# Multica agents Specification

## Purpose

Keep the agents and workers of the agents repo, `~/Repos/Agents`, equal to their folders as managed agents in the local Multica workspace, by hand, with `madspec-sync/multica_sync.py`.

## Requirements

### Requirement: Agents are Multica agents

Every folder of the agents repo with an `agent.toml` SHALL be an agent in the local Multica workspace. A folder without an `agent.toml`, such as Tika's, SHALL stay outside the sync. Each managed agent SHALL use the one local omp runtime and be open to the whole workspace. The sync SHALL never write squads.

#### Scenario: A full sync makes the agents

- **WHEN** `multica_sync.py` runs without an argument
- **THEN** the workspace holds one active managed agent per folder with an `agent.toml`, including Luuk, Tortolero, Paul, Lukas and Junior Creative
- **AND** Tika is not synced

### Requirement: An agent runs its AGENTS.md

An agent SHALL be named after its `agent.toml` `name` and carry its `title` as the description. It SHALL select its folder's `AGENTS.md` through `~/Repos/Agents/<folder>/AGENTS.md`, the single absolute path in `instruction_selection`. Its instructions SHALL be `<SystemIdentity>`, `{selection}` and `</SystemIdentity>`, each on its own line. At task start, the daemon SHALL fill `{selection}` with the file from disk, including uncommitted edits. Multica SHALL store the path, not a copy of the file. Its model and thinking level SHALL be the first entry of the `models` list and its `:level`. With more than one model, its only argument SHALL be `--config=` with a fallback overlay whose chain holds the other entries in order; with one model it SHALL have no arguments.

#### Scenario: Mitchel

- **GIVEN** Mitchel's `agent.toml` with `models = ["anthropic/claude-opus-5-5:high", "openai-codex/gpt-6.1-sol:high"]`
- **WHEN** the sync runs
- **THEN** the agent Mitchel has model `anthropic/claude-opus-5-5`, thinking level `high` and a `--config=` overlay whose chain for `anthropic/claude-opus-5-5` is `openai-codex/gpt-6.1-sol:high`

#### Scenario: A worker has one model

- **GIVEN** Medior Backend's `agent.toml` with `models = ["openai-codex/gpt-6.1-sol:high"]`
- **WHEN** the sync runs
- **THEN** the agent Medior Backend has model `openai-codex/gpt-6.1-sol`, thinking level `high` and no arguments

#### Scenario: An agent without models

- **GIVEN** an `agent.toml` without `models`
- **WHEN** the sync runs
- **THEN** it reports that folder and fails it, and every other agent is synced

#### Scenario: Guus selects his AGENTS.md

- **WHEN** the sync runs
- **THEN** Guus has `instruction_selection: ["/Users/codaveto/Repos/Agents/guus/AGENTS.md"]`
- **AND** instructions `<SystemIdentity>`, `{selection}` and `</SystemIdentity>`, each on its own line

#### Scenario: A stale selection is corrected

- **GIVEN** Guus with another instruction selection
- **WHEN** the sync runs
- **THEN** his selection is his `AGENTS.md` again

#### Scenario: An uncommitted AGENTS.md edit reaches the next task

- **GIVEN** an uncommitted edit to `guus/AGENTS.md` in the agents repo
- **WHEN** a task of Guus starts
- **THEN** his instructions hold the edited file without another sync


### Requirement: The sync manages only its own agents

An agent SHALL count as the sync's own when its env holds `MADSPEC_PERSONA` or `MADSPEC_WORKER`. A new agent SHALL get `MADSPEC_WORKER=<folder>` when its folder name starts with `junior-`, `medior-` or `senior-`, and `MADSPEC_PERSONA=<folder>` otherwise. Only a matching managed identity SHALL be updated or restored. An unmanaged same-name agent SHALL stay unchanged unless its exact name is passed through repeatable `--adopt NAME`. Adoption SHALL require exactly one unmanaged match, preserve other environment values and set the source's marker before syncing its fields. A full sync SHALL archive managed agents whose folder is gone and delete overlays no active agent names. Unreadable sources SHALL be reported and protected from archive.

#### Scenario: Lukas is adopted

- **GIVEN** one unmanaged agent named Lukas
- **WHEN** `multica_sync.py Lukas --adopt Lukas` runs
- **THEN** that agent carries `MADSPEC_PERSONA=lukas` and equals his folder

#### Scenario: An unmanaged agent stays unchanged

- **GIVEN** an unmanaged agent with a source name
- **WHEN** the sync runs without adoption
- **THEN** it reports the conflict and leaves that agent unchanged

#### Scenario: A removed folder

- **GIVEN** a synced agent or worker whose folder is removed
- **WHEN** a full sync runs
- **THEN** its agent is archived

### Requirement: The sync runs by hand

`multica_sync.py` SHALL run only when started by hand. Without an argument it SHALL sync every agent and worker. With a name or folder it SHALL sync only that source, and archive and delete nothing. A failure SHALL print what failed and exit non-zero.

#### Scenario: One agent by name

- **WHEN** `multica_sync.py Mitchel` or `multica_sync.py mitchel` runs
- **THEN** only Mitchel is synced, and no agent is archived

#### Scenario: Multica is not reachable

- **GIVEN** the Multica server is down
- **WHEN** the sync runs
- **THEN** it prints that the server is unreachable and exits non-zero

### Requirement: Configuration comes from main

The sync SHALL export each top folder's `agent.toml` and `AGENTS.md` from the agents repo on `origin/main`, whichever branch or uncommitted edits the checkout holds. A missing `origin/main` SHALL fail before API writes. `AGENTS.md` content at task start SHALL still come from disk.

#### Scenario: An uncommitted agent.toml edit

- **GIVEN** an uncommitted edit to an agent's models
- **WHEN** the sync runs
- **THEN** its agent uses the committed `origin/main` configuration

### Requirement: Dry run is read-only

`--dry-run` SHALL read source and API state and print proposed creates, updates, restores, adoptions, archives and overlay writes or deletes. It SHALL write neither API state nor overlay files, and SHALL report the same source and identity errors as a write run.

#### Scenario: Preview an adoption

- **WHEN** `multica_sync.py Lukas --adopt Lukas --dry-run` runs
- **THEN** it reports the adoption and field changes without changing Lukas or any overlay
