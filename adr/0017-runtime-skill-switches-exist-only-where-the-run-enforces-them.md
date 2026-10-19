# ADR-0017: Runtime skill switches exist only where the run enforces them

- Status: superseded by ADR-0033
- Date: 2026-10-15
- Supersedes: none

## Context

An agent's runs inherit the skills installed on its runtime machine. Brian turns them off per agent, one skill at a time and as a whole section, on the agent's Skills tab. Each provider keeps a skill out in its own way: Claude through a task-local `--settings` file, Codex through its task `config.toml`, omp through a task-local `--config` overlay with `skills.ignoredSkills`. Other providers have no such mechanism. A switch for a provider whose run ignores it would tell Brian a skill is off while the run still has it. A stored list of "every skill" would go stale as soon as a skill is installed later.

## Decision

- A runtime skill switch, section or per skill, shows and is accepted only for local runtimes of a provider whose task environment keeps a turned-off skill out. The set is one function per Go package: `runtimeSkillControlsSupported` in `server/internal/handler`, `runtimeSkillsCanDisable` in `server/internal/daemon`, which also sets each listed skill's `can_disable`.
- The switches control exactly what the daemon's runtime skill listing (`listRuntimeLocalSkills`) returns for that provider; the tab shows the same listing.
- The section setting is stored as one boolean on the agent, `inherit_runtime_skills`, apart from the per-skill list. While it is off, the daemon expands it at each run start into every skill the listing returns, and passes that list to the provider's existing enforcement. When the listing fails, the task fails before the agent starts.
- A missing section setting, from an older server or claim, counts as on.

## Consequences

- Adding a provider to the switches means adding its enforcement in `server/internal/daemon/execenv` and the provider to both functions, in one change.
- A skill installed later follows the section switch with no stored change; the listing walk runs once more per section-off task.
- The tab and the run agree because they read the same listing on the same machine, as ADR-0001 requires.
