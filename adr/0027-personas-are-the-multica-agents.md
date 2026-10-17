# ADR-0027: Personas are the Multica agents

- Status: superseded by madspec ADR-0005; supersedes ADR-0025 and ADR-0026
- Date: 2026-10-12
- Supersedes: ADR-0025, ADR-0026

## Context

ADR-0025 synced every MadSpec sub agent to Multica as a worker, and ADR-0026 had those workers build each code step and each larger verifier Fix as Multica issues. Brian decided on 2026-10-12 that no MadSpec flow creates or runs Multica work by itself, that only his personas appear in Multica, and that a persona is a skill and a Multica agent, never a subagent.

## Decision

- Each persona skill `as-*` on `origin/main` is the one source of its Multica agent in the local workspace: its name, its `SKILL.md` as instructions, and the `model` list of its `customize.toml` `[agent]` as model, thinking level and omp fallback overlay. `skills/madspec/scripts/multica_sync.py` writes them; the git hooks run it after every commit and merge. The agents it manages carry `MADSPEC_PERSONA` in their env; it leaves everything else alone.
- A persona agent runs the persona as `tools/persona.sh` starts it: its skills through their global links, no Multica skills.
- A code step is built in its persona's session by the builder subagents of `agents/`, and a larger verifier Fix by the builder subagent Sarkout starts in his session.
- `agents/` holds only the subagents a session starts; a persona has no agent file.

## Consequences

- Multica holds the persona agents only; work reaches one when Brian assigns it.
- A persona agent is changed in its skill through a merged pull request, never in Multica: the next sync overwrites an edit made there.
- Multica must be reachable for a sync; a failed sync never blocks a commit.
- A code step keeps its persona's session busy for the whole build.
