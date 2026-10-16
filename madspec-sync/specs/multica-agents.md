# Multica agents Specification

## Purpose

Keep MadSpec personas and team workers equal to their office sources as managed agents in the local Multica workspace, by hand, with `madspec-sync/multica_sync.py`. Team membership and worker file locations are defined in `~/Repos/Ventures/madspec/skills/madspec/help/workers.md`.

## Requirements

### Requirement: Personas are Multica agents

Every persona with a skill that MadSpec's `roster.py --personas` reports SHALL be an agent in the local Multica workspace. Each roster team worker SHALL also be an agent. Session helpers SHALL stay outside the sync. Each managed agent SHALL use the one local omp runtime and be open to the whole workspace. The sync SHALL never write squads.

#### Scenario: A full sync makes the persona agents

- **WHEN** `multica_sync.py` runs without an argument
- **THEN** the workspace holds one active managed agent per persona, including Luuk, Tortolero, Paul and Lukas, and per roster worker
- **AND** session helpers, such as Explorer, are not synced

### Requirement: An agent runs its persona as the persona command does

A persona's agent SHALL be named after the persona, carry the description `Runs the <skill> skill.`, and select its whole skill folder through `~/Repos/Ventures/madspec/skills/<skill>` when present, else `~/Repos/Agents/<command>/skills/<skill>`. This SHALL be the single absolute path in `instruction_selection`. Its instructions SHALL be `` `{skill-root}` is <that folder>. Your instructions are the <skill> skill below; follow it. ``, a blank line, then `{selection}`. At task start, the daemon SHALL fill `{selection}` with the folder's tree and text files from disk, including uncommitted edits. Multica SHALL store paths, not a copy of the skill. Its model and thinking level SHALL be the first entry of the persona's `[agent].model` list and its `:level`. Its omp arguments SHALL be `--skills=` with the persona's skills, left out for a persona with every skill. When the model list has more than one entry, `--config=` SHALL name an overlay in `~/.local/state/madspec/multica/<command>.yml` with `retry.modelFallback: true` and a `retry.fallbackChains` entry from the first model to the remaining entries in order. Its env SHALL hold `MADSPEC_PERSONA=<command>`. Skills SHALL be reached through `~/.agents/skills/<skill>/`.

#### Scenario: Mitchel

- **GIVEN** `as-product-manager` with `model = ["anthropic/claude-opus-5-5:high", "openai-codex/gpt-6.1-sol:high"]`
- **WHEN** the sync runs
- **THEN** the agent Mitchel has model `anthropic/claude-opus-5-5`, thinking level `high`, `--skills=` with Mitchel's skills, and a `--config=` overlay whose chain for `anthropic/claude-opus-5-5` is `openai-codex/gpt-6.1-sol:high`

#### Scenario: Skuddy has every skill

- **WHEN** the sync runs
- **THEN** the agent Skuddy has no `--skills=` argument

#### Scenario: A persona without a model

- **GIVEN** a persona whose `customize.toml` has no `[agent]` `model`
- **WHEN** the sync runs
- **THEN** it reports that persona and fails it, and every other persona is synced

#### Scenario: Guus selects his whole skill folder

- **WHEN** the sync runs
- **THEN** Guus has `instruction_selection: ["/Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer"]`
- **AND** instructions `` `{skill-root}` is /Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer. Your instructions are the as-ux-designer skill below; follow it. ``, a blank line, then `{selection}`

#### Scenario: A stale selection is corrected

- **GIVEN** Guus with another instruction selection
- **WHEN** the sync runs
- **THEN** his selection is his whole skill folder again

#### Scenario: An uncommitted skill edit reaches the next task

- **GIVEN** an uncommitted edit to `skills/as-ux-designer/SKILL.md` in the MadSpec checkout
- **WHEN** a task of Guus starts
- **THEN** his instructions hold the edited file without another sync


### Requirement: The sync manages only its own agents

An agent SHALL count as the sync's own when its env holds `MADSPEC_PERSONA` or `MADSPEC_WORKER`. Only a matching managed identity SHALL be updated or restored. An unmanaged same-name agent SHALL stay unchanged unless its exact name is passed through repeatable `--adopt NAME`. Adoption SHALL require exactly one unmanaged match, preserve other environment values and set the source's marker before syncing its fields. A full sync SHALL archive managed agents whose source is gone and delete overlays no active agent names. Unreadable sources SHALL be reported and protected from archive.

#### Scenario: Lukas is adopted

- **GIVEN** one unmanaged agent named Lukas
- **WHEN** `multica_sync.py Lukas --adopt Lukas` runs
- **THEN** that agent carries `MADSPEC_PERSONA=lukas` and equals his office persona

#### Scenario: An unmanaged agent stays unchanged

- **GIVEN** an unmanaged agent with a source name
- **WHEN** the sync runs without adoption
- **THEN** it reports the conflict and leaves that agent unchanged

#### Scenario: A removed persona

- **GIVEN** a synced persona whose `as-*` skill is removed
- **WHEN** a full sync runs
- **THEN** its agent is archived

### Requirement: The sync runs by hand

`multica_sync.py` SHALL run only when started by hand. Without an argument it SHALL sync every persona and roster worker. With a name or command it SHALL sync only that source, and archive and delete nothing. A failure SHALL print what failed and exit non-zero.

#### Scenario: One persona by name

- **WHEN** `multica_sync.py Mitchel` or `multica_sync.py mitchel` runs
- **THEN** only Mitchel is synced, and no agent is archived

#### Scenario: Multica is not reachable

- **GIVEN** the Multica server is down
- **WHEN** the sync runs
- **THEN** it prints that the server is unreachable and exits non-zero

### Requirement: Configuration comes from main

The sync SHALL export `skills/` from MadSpec and `skills/` and `agents/` from every git-backed office on `origin/main`, whichever branch or uncommitted edits the checkouts hold. It SHALL call `personas(offices_root, skills_root)` with the exported office and skills roots. Source links SHALL resolve within the snapshot. A missing `origin/main` SHALL fail before API writes. Persona skill content at task start SHALL still come from disk.

#### Scenario: An uncommitted office configuration edit

- **GIVEN** an uncommitted edit to a persona model or worker body
- **WHEN** the sync runs
- **THEN** its agent uses the office's committed `origin/main` configuration

### Requirement: Workers use their agent files

The sync SHALL read only workers named by roster teams from `agents/<worker-id>.md` in each lead's office. YAML frontmatter SHALL supply name, description, skills and ordered models. The body SHALL be the instructions, with no frontmatter or instruction selection. Primary model, effort, skill arguments and fallback overlays SHALL follow the persona rules above. Worker identity SHALL be `MADSPEC_WORKER=<worker-id>`.

#### Scenario: Junior Backend

- **GIVEN** the roster names junior-backend in Joeri's team
- **WHEN** the sync runs
- **THEN** Junior Backend uses that file's body, primary model and ordered fallbacks on the local omp runtime

### Requirement: Dry run is read-only

`--dry-run` SHALL read source and API state and print proposed creates, updates, restores, adoptions, archives and overlay writes or deletes. It SHALL write neither API state nor overlay files, and SHALL report the same source and identity errors as a write run.

#### Scenario: Preview an adoption

- **WHEN** `multica_sync.py Lukas --adopt Lukas --dry-run` runs
- **THEN** it reports the adoption and field changes without changing Lukas or any overlay
