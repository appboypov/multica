# ADR-0025: Multica workers sync from main

- Status: superseded by ADR-0027; supersedes ADR-0023
- Date: 2026-10-11
- Supersedes: ADR-0023
- Change: `multica-sync-from-main`

## Context

ADR-0023 made the agent files and skill folders the source of the Multica workers, synced by the git hooks after every commit and merge. The sync read them from the working tree it ran in, so skill text of the unmerged `visuals-first` branch reached the workers before its pull request merged. A skill Multica cannot store, `madspec-logo-design` with 1464 files, kept Design System Designer out of sync.

## Decision

- Each sub agent file in `agents/` is the one source of its Multica worker: name, description, instructions, model, thinking level, tools, skills and whether it runs with the global instructions. Each skill folder in `skills/` is the one source of its Multica skill.
- The source is those files as they are on `origin/main`, never a working tree. `skills/madspec/scripts/multica_sync.py` writes them to Multica; the git hooks run it after every commit and merge, and a run off main writes nothing new. Workers and skills it manages carry a marker; it leaves everything without a marker alone.
- A worker gets only the skills its file lists and the skills those link to directly, and no global skills.
- `tools/multica/left-out.txt` names the skill paths a Multica skill leaves out. A worker reaches a left-out path through the skill's global link, `~/.agents/skills/<skill>/`.
- An agent file leaves Multica with `multica: false`; persona agents are never workers.

## Consequences

- A worker or a managed skill is changed in this repository through a merged pull request, never in Multica: the next sync overwrites an edit made there.
- Multica must be reachable for a sync; a failed sync never blocks a commit.
- A change reaches the workers only after main is pulled into the main checkout.
- A skill too large for Multica syncs once its oversized paths are on the left-out list and its scripts reach them through the global link; workers run on the Mac that holds the main checkout.
