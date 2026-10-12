# Multica agents Specification

## Purpose

Keep the MadSpec personas equal to their skills as agents in the local Multica workspace, by hand, with `madspec-sync/multica_sync.py`. Only the personas are Multica agents.

## Requirements

### Requirement: Personas are Multica agents

Every persona MadSpec's `roster.py --personas` reports SHALL be an agent in the local Multica workspace, and no sub agent in MadSpec's `agents/` SHALL be one. Each agent SHALL be bound to the one local omp runtime and open to the whole workspace.

#### Scenario: A full sync makes the persona agents

- **WHEN** `multica_sync.py` runs without an argument
- **THEN** the workspace holds one active agent per persona, such as Mitchel, Pieter and Skuddy
- **AND** no sub agent, such as Explorer, is synced

### Requirement: An agent runs its persona as the persona command does

A persona's agent SHALL be named after the persona, carry the description `Runs the <skill> skill.`, and select its whole skill folder in the MadSpec checkout, `~/Repos/Ventures/madspec/skills/<skill>`, as the single absolute path in `instruction_selection`. Its instructions SHALL be `` `{skill-root}` is <that folder>. Your instructions are the <skill> skill below; follow it. ``, a blank line, then `{selection}`. At every task start, the daemon SHALL fill `{selection}` with the folder's tree and text files as they are on disk, including uncommitted edits. Multica SHALL store paths, not a copy of the skill. Its model and thinking level SHALL be the first entry of the `model` list in the persona's `customize.toml` `[agent]` and its `:level`. Its omp arguments SHALL be `--skills=` with the persona's skills, left out for a persona with every skill, and, when `model` lists more than one entry, `--config=` with an overlay in `~/.local/state/madspec/multica/<command>.yml` that sets `retry.modelFallback: true` and a `retry.fallbackChains` entry from the first model to the other entries in order. Its env SHALL hold `MADSPEC_PERSONA=<command>`. The persona SHALL reach its skills through their global links, `~/.agents/skills/<skill>/`.

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

An agent SHALL count as the sync's own when its env holds `MADSPEC_PERSONA`. An unmarked agent with a persona's name SHALL be taken over and given the marker, an active one before an archived one. An archived agent of a persona SHALL be restored. A full sync SHALL archive its agents whose persona is gone and delete the overlays no active agent names. Agents without the marker SHALL stay as they are.

#### Scenario: An archived persona agent is taken over

- **GIVEN** the archived, unmarked agent Mitchel
- **WHEN** the sync runs
- **THEN** the same agent is restored, carries `MADSPEC_PERSONA=mitchel` and equals the persona

#### Scenario: A removed persona

- **GIVEN** a synced persona whose `as-*` skill is removed
- **WHEN** a full sync runs
- **THEN** its agent is archived

### Requirement: The sync runs by hand

`multica_sync.py` SHALL run only when started by hand. Without an argument it SHALL sync every persona. With a persona name or command it SHALL sync only that persona, and archive and delete nothing. A failure SHALL print what failed and exit non-zero.

#### Scenario: One persona by name

- **WHEN** `multica_sync.py Mitchel` or `multica_sync.py mitchel` runs
- **THEN** only Mitchel is synced, and no agent is archived

#### Scenario: Multica is not reachable

- **GIVEN** the Multica server is down
- **WHEN** the sync runs
- **THEN** it prints that the server is unreachable and exits non-zero

### Requirement: Persona configuration comes from main

The sync SHALL read the persona list, models and arguments from `skills/` on `origin/main` in the MadSpec checkout `~/Repos/Ventures/madspec`, whichever branch or uncommitted change the checkout holds. The selected skill content SHALL come from that checkout's disk at task start, not from the sync's exported `origin/main` tree. Without an `origin/main` the sync SHALL print that and exit non-zero, writing nothing.

#### Scenario: An uncommitted edit in the MadSpec checkout

- **GIVEN** the MadSpec checkout on main with an uncommitted edit to a persona's `customize.toml`
- **WHEN** the sync runs
- **THEN** the agent's model and arguments come from that `customize.toml` as it is on `origin/main`
