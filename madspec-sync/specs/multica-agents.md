# Multica agents Specification

## Purpose

Keep the agents and workers of the agents repo, `~/Repos/Agents`, equal to their folders as managed agents in the local Multica workspace, each running on its own runtime profile through its alias, by hand, with `madspec-sync/multica_sync.py`.

## Requirements

### Requirement: Agents are Multica agents

Every folder of the agents repo with an `agent.toml` SHALL be an agent in the local Multica workspace. A folder without an `agent.toml`, such as Tika's, SHALL stay outside the sync and get no runtime profile. Each managed agent SHALL run on its runtime profile and be open to the whole workspace. Agents the sync does not manage, such as Herdr, Astra, Fable and Haiku, SHALL keep their runtimes. The sync SHALL never write squads.

#### Scenario: A full sync makes the agents

- **WHEN** `multica_sync.py` runs without an argument
- **THEN** the workspace holds one active managed agent per folder with an `agent.toml`, including Luuk, Tortolero, Paul, Lukas and Junior Creative
- **AND** Tika is not synced

### Requirement: Each agent runs on its runtime profile

Every folder with an `agent.toml` SHALL have a runtime profile whose `command_name` is the folder name, the alias in `~/.local/bin`, and whose runtime type is omp. The sync SHALL create it as `POST /api/workspaces/<workspace>/runtime-profiles` with the agent's name as `display_name` when no profile has that `command_name`, and reuse it otherwise. It SHALL pin the profile's executable with `multica [--profile <daemon profile>] runtime profile set-path <profile id> --path ~/.local/bin/<folder>` when the `profile_command_overrides` of that profile's CLI config does not already hold that path. That config is `~/.multica/config.json` for the default profile and `~/.multica/profiles/<profile>/config.json` otherwise. A changed pin SHALL print that the daemon needs a restart for the path to take effect. The agent's `runtime_id` SHALL be the profile's local runtime on the running daemon. When that runtime is missing, only that agent SHALL fail, with "restart the daemon, then run the sync again".

#### Scenario: Mitchel's profile

- **WHEN** the sync runs
- **THEN** a runtime profile with command `mitchel` and runtime type omp exists, pinned to `~/.local/bin/mitchel` for the daemon's profile
- **AND** the agent Mitchel runs on its runtime

#### Scenario: A deleted profile

- **GIVEN** the profile `guus` was deleted
- **WHEN** the sync runs
- **THEN** the profile `guus` exists again and Guus runs on it

#### Scenario: A profile the daemon has not registered

- **GIVEN** the profile `guus` without a runtime on the running daemon
- **WHEN** the sync runs
- **THEN** Guus is reported as failed with "restart the daemon, then run the sync again" and every other agent is synced

### Requirement: The sync works against the running daemon

The running daemon SHALL be the one `multica daemon status --output json` reports for the default CLI profile when it is `running` against the sync's server and holds its workspace. Otherwise the sync SHALL ask `multica --profile <profile> daemon status --output json` for each folder in `~/.multica/profiles` and take the one daemon running against that server and workspace. With no such daemon, or more than one, the whole sync SHALL fail before any write, overlay or path pin.

#### Scenario: The desktop daemon

- **GIVEN** the desktop daemon runs under the CLI profile `desktop-localhost-8080` and no daemon runs under the default profile
- **WHEN** the sync runs
- **THEN** the profile paths are pinned in `~/.multica/profiles/desktop-localhost-8080/config.json`

#### Scenario: No daemon runs

- **WHEN** the sync runs while no daemon serves its server and workspace
- **THEN** it prints that the daemon is not running and exits non-zero, writing nothing

### Requirement: An agent runs through its alias

An agent SHALL be named after its `agent.toml` `name` and carry its `title` as the description. Its `instructions` SHALL be empty and its `instruction_selection` `[]`: its runtime profile starts the alias, which loads the folder's `AGENTS.md` from disk at each start, including uncommitted edits. With more than one model, its only argument SHALL be `--config=` with a fallback overlay whose chain holds the other entries in order; with one model it SHALL have no arguments.

#### Scenario: Mitchel

- **GIVEN** Mitchel's `agent.toml` with `models = ["anthropic/claude-opus-5-5:high", "openai-codex/gpt-6.1-sol:high"]`
- **WHEN** the sync runs
- **THEN** the agent Mitchel has empty instructions, no selection and a `--config=` overlay whose chain for `anthropic/claude-opus-5-5` is `openai-codex/gpt-6.1-sol:high`

#### Scenario: A worker has one model

- **GIVEN** Medior Backend's `agent.toml` with `models = ["openai-codex/gpt-6.1-sol:high"]`
- **WHEN** the sync runs
- **THEN** the agent Medior Backend has no arguments

#### Scenario: An agent without models

- **GIVEN** an `agent.toml` without `models`
- **WHEN** the sync runs
- **THEN** it reports that folder and fails it, and every other agent is synced

#### Scenario: A stale selection is cleared

- **GIVEN** Guus with an instruction selection, instructions and `--skills=`
- **WHEN** the sync runs
- **THEN** they are cleared and his `--config=` overlay stays

#### Scenario: An uncommitted AGENTS.md edit reaches the next task

- **GIVEN** an uncommitted edit to `guus/AGENTS.md` in the agents repo
- **WHEN** a task of Guus starts
- **THEN** his alias loads the edited file without another sync

### Requirement: Multica owns an agent's model after its creation

A new agent SHALL start with the first entry of its `models` list as its model and that entry's `:level` as its thinking level. Updates of an existing agent SHALL leave out `model` and `thinking_level`, so a model or thinking level set in Multica survives every later sync, whatever `agent.toml` says.

#### Scenario: Mitchel is created, then edited

- **GIVEN** no agent Mitchel and `models = ["anthropic/claude-opus-5-5:high", …]`
- **WHEN** the sync runs
- **THEN** the agent Mitchel has model `anthropic/claude-opus-5-5` and thinking level `high`
- **AND** after Brian sets another model and thinking level in Multica and Mitchel's `models` change, the next sync leaves both as Brian set them

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

The sync SHALL export each top folder's `agent.toml` and `AGENTS.md` from the agents repo on `origin/main`, whichever branch or uncommitted edits the checkout holds. A missing `origin/main` SHALL fail before API writes. The alias SHALL still read `AGENTS.md` from disk at each start.

#### Scenario: An uncommitted agent.toml edit

- **GIVEN** an uncommitted edit to an agent's models
- **WHEN** the sync runs
- **THEN** its agent uses the committed `origin/main` configuration

### Requirement: Dry run is read-only

`--dry-run` SHALL read source, daemon and API state and print proposed runtime profile creates, path pins, agent creates, updates, restores, adoptions, archives and overlay writes or deletes. It SHALL write neither API state, path pins nor overlay files, and SHALL report the same source, identity and missing-runtime errors as a write run.

#### Scenario: Preview an adoption

- **WHEN** `multica_sync.py Lukas --adopt Lukas --dry-run` runs
- **THEN** it reports the adoption and field changes without changing Lukas or any overlay
