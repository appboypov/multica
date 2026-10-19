## At a glance

A decider rule can be disabled. A disabled rule stays in the workspace policy and starts no decision.

## Why

The squad rules of Brian's workspace ask "Which squad takes this work?" on every issue that enters Todo or joins a release. That question is too vague for now. Success means these rules ask nothing while they stay in the policy for a later look.

## What Changes

- A decider rule takes an optional `disabled` flag.
- The decider skips disabled rules on every event.
- Disabled rules still pass the policy's validation, so they stay ready to turn on again.
- A data migration sets `disabled: true` on `todo-squad` and `release-squad` in workspace `30e8e92f-9a49-4167-95e1-195ab4ec38be`. It preserves `post-merge-bug` and every other setting.

## Capabilities

### Modified Capabilities

- `routine-decider`: a rule can be disabled without removing it.

## Impact

The decider service and its business-logic tests change. A workspace-scoped data migration applies Brian's requested policy. The settings API and its authorization stay unchanged.

## Constraints

- A disabled rule stays in `settings.decider.rules` with all its fields.
- Policy changes stay Brian's own.

## Non-goals

No switch for the whole decider. No settings screen. No change to open routine questions.

## Sources

- WORK-1779.
- `server/internal/service/decider.go`
- `server/internal/service/decider_judgment.go`
