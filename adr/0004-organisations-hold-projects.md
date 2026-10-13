# ADR-0004: Organisations hold projects, and an organisation's key names its worktree folder

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

Brian steers his own work and the work of other organisations from one Multica workspace (PRD: Multica: steer projects, repositories, worklanes and releases). Eight journeys build on one model:

| Journey | Adds |
|---|---|
| UJ-1 | Organisations and their projects |
| UJ-2 | A project's repositories, and worktrees at `~/Worktrees/{organisation key}/{repo}/{folder}` |
| UJ-3 | A repository's worklanes |
| UJ-4 | A repository's release line |
| UJ-5 to UJ-8 | Issues placed in worklanes and releases, and the pages and shipping built on them |

A Multica workspace is the tenant and holds every member, agent and runtime. Splitting it per organisation would split Brian's agents and inbox.

## Decision

An organisation is a workspace-scoped table: `organisation(id, workspace_id, name, key, created_at, updated_at)`.

- `name` is unique per workspace, ignoring case.
- `key` is unique per workspace and matches `^[a-z0-9][a-z0-9-]*$`.
- `project.organisation_id` is a nullable soft reference with no foreign key, so a project has at most one organisation.

The chain is:

```
workspace → organisation → project → repository → worklane and release → issue
```

Each later level hangs from the project or from the level below it, never from the organisation directly. The organisation's `key` is the only part of the organisation that code outside the UI reads: it is the first folder of every worktree path.

There is no rename or delete of an organisation. An organisation's key can change. Worktrees made before a key change stay where they are, and new worktrees use the new key.

## Consequences

- UJ-2 adds a repository table keyed by `project_id`. It reads the organisation's key through the project, and a project without an organisation has no worktree folder until it gets one.
- Worklanes and releases reference a repository, never an organisation or a project directly, so moving a project to another organisation moves everything under it.
- Counts per organisation (projects, repositories, worklanes) are computed from the lists the client already loads, or by a query joined through `project`. There is no denormalised counter.
- Deleting a workspace deletes its organisations in application code, in the step that deletes its projects.
- Brain concepts stay out of the schema. The brain sync keeps its own map from brain ids to Multica ids.
