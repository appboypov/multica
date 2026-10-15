## Outcome

On an agent's Capabilities > Skills tab, the "Inherited from runtime" section gets one switch in its header, and every inherited skill row keeps or gets its own switch, for Claude, Codex and now omp agents. Off on the section: the agent's runs get none of the skills the tab lists, only its assigned skills. Per-skill choices are kept while the section is off and come back when it is on again.

## Core idea

One rule for all three providers: **what the tab lists is what the switches control.**

- The section switch is one agent field, `inherit_runtime_skills` (default `true`).
- At each run, when it is `false`, the daemon expands it into "every runtime skill the tab lists is disabled", using the same discovery walk the tab uses (`listRuntimeLocalSkills`). That list feeds the existing enforcement of each provider. So a skill that appears in the runtime later is out at the next run, with no stored change.
- When it is `true`, the run uses the stored per-skill list, as today.
- omp gets enforcement through a task-local omp settings file with `skills.ignoredSkills`, passed with `--config`, the same way Claude gets its task-local `--settings` file.
- An assigned skill with the same name as a turned-off runtime skill still reaches the run: every provider path keeps skipping names that `workspaceClaimsRuntimeSkill` matches.

Why this way: no second enforcement mechanism per provider, no stored "all skills" list that goes stale, and the UI and the run agree because both read the same discovery.

## Per area

### Database

- **Today:** `agent.disabled_runtime_skills JSONB NOT NULL DEFAULT '[]'` (migration `206`). No section flag.
- **Will:** new migration `711_agent_inherit_runtime_skills.{up,down}.sql`: `ALTER TABLE agent ADD COLUMN inherit_runtime_skills BOOLEAN NOT NULL DEFAULT true`; down drops it. Existing and new agents inherit. No index, no foreign key.
- **Queries:** `server/pkg/db/queries/agent.sql` gets `UpdateAgentInheritRuntimeSkills`; `make sqlc` regenerates `server/pkg/db/generated/*`. `GetAgent`/lists use `SELECT *`, so the field flows by itself.

### API (`server/internal/handler`)

- **Today:** `PUT /api/agents/{id}/runtime-skills/enabled` (`agent_runtime_skills.go`) stores per-skill overrides; it answers 400 unless the runtime is local and `codex` or `claude` (line 119).
- **Will:**
  - New route `PUT /api/agents/{id}/runtime-skills/inherit` with `{ "enabled": bool }`, in `agent_runtime_skills.go`, beside the per-skill route in `server/cmd/server/router.go`. Same permission check, same agent row lock, same `agent:status` broadcast, 204. It never touches `disabled_runtime_skills`, so per-skill choices survive.
  - One helper `runtimeSkillControlsSupported(provider)` = `claude`, `codex`, `omp`, used by both routes. The per-skill route then accepts omp (plugin identity stays Claude-only).
  - `AgentResponse` (`agent.go`) gets `inherit_runtime_skills`.
  - The claim payload (`daemon.go` ~2630, `TaskAgentData`) gets `inherit_runtime_skills` next to the scoped `disabled_runtime_skills`.
- Agent create, update and duplicate do not set or copy the field, the same as `disabled_runtime_skills` today.

### Daemon (`server/internal/daemon`)

- **Today:** `local_skills.go:619` sets `CanDisable: provider == "codex" || provider == "claude"`. `convertDisabledRuntimeSkillsForEnv` (`daemon.go` ~9853) passes only the stored list, scoped to the claimed runtime and provider.
- **Will:**
  - `CanDisable` comes from one function `runtimeSkillsCanDisable(provider)` = `claude`, `codex`, `omp`.
  - `AgentData` (`types.go`) gets `InheritRuntimeSkills *bool`; a missing value counts as `true`, so a payload without it never turns skills off.
  - `convertDisabledRuntimeSkillsForEnv` takes the section flag into account: when it is off and the provider is in the supported set, it returns every skill `listRuntimeLocalSkills(provider)` finds (root, key, name, plugin). A discovery error fails the task preparation, as a failed Claude settings write does today, so a run never silently gets skills that are turned off.

### Task environment (`server/internal/daemon/execenv`)

- **Today:** Claude through `prepareClaudeSkillSettings` (deny rules file), Codex through `hydrateCodexSkills` + `ensureCodexDisabledSkillsConfig`. omp only gets `.omp/skills` (assigned skills) and `.omp/mcp.json`; the disabled list is ignored.
- **Will:** new `prepareOmpSkillSettings(envRoot, disabled, workspaceSkills)` in `runtime_skill_policy.go`, called for `omp` in both `Prepare` and the `Reuse` refresh (`execenv.go` ~567 and ~864):
  - writes `{envRoot}/omp-runtime-skill-settings.yml` with `skills: { ignoredSkills: [...] }`, one entry per disabled skill name, glob characters escaped (omp matches entries as globs on the skill name);
  - skips names `workspaceClaimsRuntimeSkill` matches;
  - removes the file when nothing is left, as the Claude function does;
  - stores the path in `env.OmpSettingsPath`.

### omp launch (`server/pkg/agent`)

- **Today:** `buildPiArgs` (`pi.go`) builds `-p --mode json [--session] [--model] [--thinking] …custom_args`; no `--config`. `--no-skills` is blocked in custom args (it would also drop assigned skills).
- **Will:** `ExecOptions` (`agent.go`) gets `OmpSettingsPath`; the daemon sets it from the env (`daemon.go` ~8340, next to `ClaudeSettingsPath`); `buildPiArgs` appends `--config <path>` last, only for omp. omp (v18.10.1, installed) merges `--config` files over project and global settings, later files win.
- **Known limit:** omp replaces a list setting rather than merging it, so while this file exists it replaces any `skills.ignoredSkills` set in the user's own omp config for that run. Brian's `~/.omp/agent/config.yml` sets none today.

### Web and desktop UI (`packages/views/agents/components/tabs/skills-tab.tsx`)

- **Today:** two `CapabilitySection`s; the inherited one has only a Refresh action. A row switch shows when `canEdit && skill.can_disable === true && skill.root`; its state comes from `isRuntimeSkillDisabled(agent.disabled_runtime_skills, …)`; it calls `api.setAgentRuntimeSkillEnabled` then refreshes the agent.
- **Will:**
  - A `Switch` in the inherited section's header, before Refresh, shown when `canEdit` and at least one listed skill has `can_disable`. Checked = `agent.inherit_runtime_skills !== false`. It calls a new `api.setAgentRuntimeSkillsInherited(agentId, enabled)`, with the same busy spinner, toast and agent refresh as the row switches.
  - While the section is off, row switches still render but show off and are disabled; turning the section on shows their stored state again.
  - omp rows get switches by themselves once the daemon reports `can_disable`.
- **Edge:** with an empty skill list the header switch is hidden, since nothing is listed to control; a stored "off" still holds and shows again once a skill appears.

### Shared client (`packages/core`)

- `types/agent.ts`: `inherit_runtime_skills?: boolean` on `Agent` (absent = on, for older servers).
- `api/client.ts`: `setAgentRuntimeSkillsInherited`, `PUT …/runtime-skills/inherit`.

### Copy

- `packages/views/locales/{en,fr,ja,ko,zh-Hans}/agents.json`, `tab_body.skills`: `runtime_inherit_aria` ("Inherit skills from runtime") and `runtime_inherit_failed_toast`. No new description text: the section title already says what the switch controls.
- Docs `apps/docs/content/docs/skills.mdx` (+ `fr`, `ja`, `ko`, `zh`), line 96: "Claude Code and Codex" becomes Claude Code, Codex and Oh-My-Pi, plus one sentence that the whole section can be turned off.

### Not touched

- Mobile shows no runtime skills.
- `multica agent get --output json` shows the new field by itself; no new CLI command.
- Agent copy (`cmd_agent_copy.go`, `packages/core/agents/draft.ts`) keeps not copying runtime-skill choices.
- Other providers keep no switches, since nothing enforces them.

## How each result is checked

| Result | Check |
|---|---|
| Column default on, existing agents inherit | Migration up/down in the worktree database; `GET` agent shows `inherit_runtime_skills: true` |
| Section route stores, keeps per-skill list, broadcasts, rejects other providers | Handler tests next to `skill_list_test.go` and `agent_runtime_skills_broadcast_test.go` |
| Per-skill route accepts omp | Handler test |
| omp rows report `can_disable` | `local_skills_test.go` case for omp |
| Section off expands to every listed skill; missing flag counts as on | `daemon_test.go` next to `TestConvertDisabledRuntimeSkillsForEnvScopesToClaimedRuntime`, with a temp HOME holding skills |
| omp settings file: names, escaping, assigned-name skip, removal | `runtime_skill_policy_test.go` |
| `--config` appended last, omp only | `pi_test.go` / `omp_test.go` |
| Header switch, disabled rows while off, restored state, omp rows | `skills-tab.test.tsx` |
| End to end | Worktree stack (`make up`) with a local omp agent: turn one skill off, run a task, read the task's `omp-runtime-skill-settings.yml` and the run's skill list; turn the section off, run again, see only assigned skills; turn it on, see the single-skill choice back. Same section-off run once for a Claude agent. |

Pieter
