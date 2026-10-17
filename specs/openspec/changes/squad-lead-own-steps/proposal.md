## At a glance

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  trigger[Leader trigger] --> choose[Read issue and Agent Identity]:::changed
  choose -->|Leader's step| own[Do own role step]:::new
  choose -->|Member fits better| delegate[Mention suitable member]
  own --> next[Assess next step]:::new
  next -->|Member fits better| delegate
  next -->|No delegation| deliver[Deliver own result and apply status rules]:::new
  delegate --> stop[Record activity and end turn]:::changed
  deliver --> stop
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Handler protocol | `squadOperatingProtocolHeader`, `squadOperatingProtocolHardRules` | changed | `server/internal/handler/squad_briefing.go` |
| Handler test | Squad Operating Protocol tests | changed | `server/internal/handler/squad_briefing_test.go` |
| Handler test | Parent status contract test | changed | `server/internal/handler/squad_parent_status_contract_test.go` |
| Runtime brief | `writeWorkflowIssue` leader status bullet | changed | `server/internal/daemon/execenv/runtime_config_sections.go` |
| Runtime brief test | Leader issue workflow tests | changed | `server/internal/daemon/execenv/runtime_config_test.go` |
| Daemon test | `TestBuildPromptSquadLeaderOwnWorkRequiresReply` | changed | `server/internal/daemon/prompt_test.go` |
| Builtin reference | Squads reference | changed | `server/internal/service/builtin_skills/multica-platform/references/squads.md` |
| Builtin reference | Issues reference | changed | `server/internal/service/builtin_skills/multica-platform/references/issues.md` |
| Builtin skill test | `TestPlatformSkillCoversPlatformContracts` | changed | `server/internal/service/builtin_skills_test.go` |
| Guide | Squads guide, English | changed | `apps/docs/content/docs/squads.mdx` |
| Guide | Squads guide, Chinese | changed | `apps/docs/content/docs/squads.zh.mdx` |
| Guide | Squads guide, French | changed | `apps/docs/content/docs/squads.fr.mdx` |
| Guide | Squads guide, Japanese | changed | `apps/docs/content/docs/squads.ja.mdx` |
| Guide | Squads guide, Korean | changed | `apps/docs/content/docs/squads.ko.mdx` |
| OpenSpec | Change metadata | new | `specs/openspec/changes/squad-lead-own-steps/.openspec.yaml` |
| OpenSpec | Proposal | new | `specs/openspec/changes/squad-lead-own-steps/proposal.md` |
| OpenSpec | Design | new | `specs/openspec/changes/squad-lead-own-steps/design.md` |
| OpenSpec | Tasks | new | `specs/openspec/changes/squad-lead-own-steps/tasks.md` |
| OpenSpec | `squad-leader-steps` spec delta | new | `specs/openspec/changes/squad-lead-own-steps/specs/squad-leader-steps/spec.md` |

## Why

Squad leads have persona duties as well as routing duties. A blanket ban on doing the work blocks their own decisions and outputs. A lead must be able to do its role step, then pass the next step to a suitable member.

## What changes

- The leader reads the issue and its Agent Identity, including its persona skill, on every trigger.
- It does its own steps in the current turn and delegates member steps by the existing mention path.
- The stop rule applies after delegation, not before own work.
- Own work without delegation uses normal result delivery and status rules.

The full behavior is in the [spec delta](specs/squad-leader-steps/spec.md). The [design](design.md) names the affected prompt surfaces and boundaries.

## Capabilities

### Added capabilities

- `squad-leader-steps`: choose own work or delegation on each leader trigger.

### Modified capabilities

None.

## Impact

The server's squad briefing, the daemon's leader status guidance, prompt and builtin skill contract tests, builtin squad and issue references, and squad guides in all five languages. No API, database, trigger, squad member behavior, or live data changes.
