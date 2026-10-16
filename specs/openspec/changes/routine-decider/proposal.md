# Routine decisions

## At a glance

Configured workspace rules make small choices on issue status and release events. Jev answers first. A Decider agent on Sonnet 5.5 handles uncertain answers. The server applies both through one path and records the result on the issue.

## Why

Routine bug triage and release checks need a clear decision without starting a full lead session each time.

## What

- Match status-entry and release-join events without polling.
- Ask the TypeSafe choice API with a confidence gate.
- Queue a named Decider agent when confidence is low.
- Route post-merge bugs to a lead or Brian's to-do inbox.
- Ask Brian to check a release only when all its issues are done and the decision is ready.
- Keep the actor, confidence and reason in issue comments.

## Scope

The settings contract and operation are in [design.md](design.md). This change depends on to-do inbox PR 49. It makes no live workspace changes.
