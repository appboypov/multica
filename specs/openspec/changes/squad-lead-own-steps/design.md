## At a glance

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  subgraph server[Go server]
    protocol[squad_briefing.go]:::changed
    claim[Leader claim]
    protocol --> claim
  end
  subgraph daemon[Daemon]
    workflow[runtime_config_sections.go]:::changed
    turn[Leader turn]
    workflow --> turn
  end
  claim -->|Instructions| turn
  turn -->|Leader's step| own[Own role step]:::new
  turn -->|Member fits better| delegate[Mention member]
  own --> next[Assess next step]:::new
  next -->|Member fits better| delegate
  next -->|No delegation| deliver[Deliver result and apply status rules]:::new
  delegate --> stop[Record activity and end turn]:::changed
  deliver --> stop
```

| Surface | Responsibility |
|---|---|
| `squadOperatingProtocolHeader` | Step selection, own work, delegation, activity, stop rule |
| `squadOperatingProtocolHardRules` | Identity limits, roster mentions, one comment, no duplicate dispatch |
| `writeWorkflowIssue` | Result delivery and status facts within protocol authority |
| Builtin reference and squad guides | Explain the behavior without runtime configuration |

## Context

A claim appends the Squad Operating Protocol to the leader's instructions. The daemon adds issue workflow guidance. These two surfaces must agree. Agent Identity remains the authority for allowed actions. The owning and guest leader status rules remain separate.

## Decisions

### Choose the next step within the leader's identity

The protocol asks the leader to read the issue and its persona skill on every trigger. It handles its own role steps and compares each next step with member skills and roles. There is no mode, setting, or runtime branch for this choice. The [spec delta](specs/squad-leader-steps/spec.md) defines the cases.

### Stop after delegation

The leader can do its own step and then delegate in the same turn. Its single comment carries its result and the short delegation. After recording squad activity, the turn ends. A turn with no delegation delivers a result normally. Activity and the silent `no_action` rule stay in the protocol, not in a second daemon rule.

### Keep status authority separate

Dispatch leaves an owned parent in `in_progress`. The daemon does not require a later member trigger before own work can meet the completion rule. It points to the protocol for status authority. The owned-parent and guest-parent instruction blocks do not change.

### Keep triggers and members unchanged

Member updates, member completion, and mentions keep their current routing. Only leader instructions change. No trigger service, task wire field, squad member instruction, or database field changes.

## Risks

- Prompt text guides a model; unit tests can check the instruction contract, not prove every model follows it.
- The two prompt surfaces could disagree on completion. Composition tests cover both together.
- Open PR 53 (`work-1042`, sub-squads backend) and PR 81 (`work-1157`, release coordination) also touch `squad_briefing.go`. Resolve their shared hunks while keeping both contracts. PR 55 (`work-1042-ui`) does not touch this file.

## Verification

No checks run for this assignment. The [tasks](tasks.md) name the deferred Crabbox checks. No migration or live rollout step is needed.
