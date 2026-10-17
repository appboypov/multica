## At a glance

```mermaid
flowchart LR
  protocol[Leader protocol] --> workflow[Runtime workflow]
  workflow --> contracts[Prompt contract tests]
  contracts --> docs[Squad references]
  docs --> remote[Deferred Crabbox checks]
```

| Area | Delivery |
|---|---|
| Handler and daemon | Own steps and delegation agree |
| Contract tests | Own work, dispatch stop, status boundaries |
| Docs | Builtin reference and all squad guide languages |
| Verification | Parent runs checks on the PR head |

## 1. Implementation

- [x] 1.1 Update `squad_briefing.go` to choose own steps or member delegation on every trigger while keeping activity and status boundaries.
- [x] 1.2 Update the leader bullet in `writeWorkflowIssue` so own delivery can complete without a member re-trigger.
- [x] 1.3 Update handler, daemon, and execenv prompt contract tests, including the composed status contract.
- [x] 1.4 Update builtin `references/squads.md` and all five `apps/docs/content/docs/squads*.mdx` guides.

## 2. Deferred verification

No checks run in this assignment, as requested. Run these through Crabbox on the PR head:

- [ ] 2.1 Handler tests: `TestSquadOperatingProtocolScopesParentStatusOwnership`, `TestSquadOperatingProtocolAllowsOwnStepsBeforeDelegation`, `TestSquadOperatingProtocolOwnsNoActionRule`, `TestSquadAssignedLeaderCanWrapUpOnCommentTurn`, and `TestGuestLeaderCannotChangeStatusOnCommentTurn`. The handler test package needs its normal migrated test database, including for protocol-only tests.
- [ ] 2.2 Daemon tests: `TestBuildPromptSquadLeaderOwnWorkRequiresReply`, `TestBuildPromptSquadLeaderReplyCarveOutIgnoresTriggerAuthor`, and `TestBuildPromptProtocolHeadingInInstructionsIsNotALeader`.
- [ ] 2.3 Execenv tests: `TestSquadLeaderIssueWorkflowKeepsParentInProgress`, `TestIssueWorkflowHonorsAgentIdentity`, and `TestProtocolHeadingInInstructionsGetsNoLeaderBrief`.
- [ ] 2.4 Check Go formatting for the touched Go files and Markdown formatting for the docs.
- [ ] 2.5 Run `cd specs && npx --yes @fission-ai/openspec@1.13.0 validate squad-lead-own-steps --type change --strict`.
