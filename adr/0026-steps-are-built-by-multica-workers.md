# ADR-0026: Steps are built by Multica workers

- Status: superseded by ADR-0027
- Date: 2026-10-11
- Change: `multica-workers-build-code-steps`

## Context

A code step (L6; J1, J2, J8, J9, J10) was built inside its persona's session: by builder subagents, or directly by the persona ("Implement directly", the oneshot route). The session was busy for the whole build, and a builder ran only on the first model of its agent file. Every agent file is a Multica worker since ADR-0023, synced from `origin/main` since ADR-0025, and every repository is a Multica project in worktree mode since ADR-0024. Brian's Approach "Multica workers build the code steps" settled how the two meet.

## Decision

- The build of a code step runs as Multica sub-issues of one step issue: one builder sub-issue per `tasks.md` group, one stage at a time in `tasks.md` order. The step itself stays in the shapeshifter's session; its persona merges the workers' branches into the step branch, runs the change's tests and opens the pull request.
- No review runs in a build: no reviewer workers, no review of any kind. Reviewing is Sarkout's alone, at his verification, with `madspec-local-review`.
- Nothing is built in the persona's session: no oneshot route, no direct build, no builder subagent. A missing worker, a blocked sub-issue or an unreachable Multica makes the build wait.
- A Fix goes to the worker that built that part. A worker is changed only through its agent file and skills on `main`, never in its result or its Multica record; a split is Brian's decision.
- A worker runs on its agent file's models in order: the sync gives it an omp `--config` overlay with `retry.fallbackChains`, derived from `model:` and kept outside the repository.
- Events reach a shapeshifter only through the listen tools.

## Consequences

- A code step depends on Multica and its daemon being up; without them the build waits.
- Builds take one stage at a time plus Multica's start-up, while the persona's session is free for other work.
- A worker's quality problem is fixed in its skill or tooling, so every later build gets the fix.
- Multica and omp stay unchanged; the step branch reaches a worker through the shared refs of the repository.
- Design and prototype steps (J3 to J7) still run in the persona's session until a later change.
