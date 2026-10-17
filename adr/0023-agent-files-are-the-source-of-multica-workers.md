# ADR-0023: Agent files are the source of Multica workers

- Status: superseded by ADR-0025
- Date: 2026-10-09
- Change: `sync-sub-agents-to-multica-workers`

## Context

The personas in Herdr hand Tasks to role workers in Multica. A worker made by hand in Multica drifts from the agent file the same role runs from in Herdr, and its skills drift from their folders. Brian decided on 2026-10-09 that every MadSpec sub agent becomes a Multica worker that matches its source.

## Decision

- Each sub agent file in `agents/` is the one source of its Multica worker: name, description, instructions, model, thinking level, tools, skills and whether it runs with the global instructions. Each skill folder in `skills/` is the one source of its Multica skill.
- `skills/madspec/scripts/multica_sync.py` writes them to Multica, and the git hooks run it after every commit and merge. Workers and skills it manages carry a marker; it leaves everything without a marker alone.
- A worker gets only the skills its file lists and the skills those link to directly, and no global skills.
- An agent file leaves Multica with `multica: false`; persona agents are never workers.

## Consequences

- A worker or a managed skill is changed in this repository, never in Multica: the next sync overwrites an edit made there.
- Multica must be reachable for a sync; a failed sync never blocks a commit.
- A skill Multica cannot store keeps every worker that needs it out of sync until the skill or the worker's list changes.
