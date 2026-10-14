# issue-worktrees Specification

## Purpose
Lets an agent work an issue without a worklane in a worktree per repository under `~/Worktrees`, leaving Brian's checkouts untouched.

## Requirements

### Requirement: An agent works an issue in a worktree per repository
An agent's task on an issue without a worklane SHALL get the repositories of the issue's project, and each repository it works in SHALL be a git worktree of that repository's checkout at `~/Worktrees/{organisation key}/{repo}/{issue key}` on the checkout's runtime, with `{repo}` the repository's name and `{issue key}` in lowercase, on the issue's branch cut from `origin/{default branch}`. For a project without an organisation the path SHALL be `~/Worktrees/{repo}/{issue key}`. Every task on the same issue SHALL work in the same worktree. The task MUST NOT change the repository's normal checkout.

#### Scenario: Multica's issue
- **GIVEN** the project Multica of De App Specialist, key `das`, holds appboypov/multica with its checkout `~/Repos/multica` on Brian's Mac
- **WHEN** an agent on Brian's Mac works the issue MUL-12 of Multica, which has no worklane
- **THEN** it works in `~/Worktrees/das/multica/mul-12`, a worktree of `~/Repos/multica` cut from `origin/main`
- **AND** `~/Repos/multica` keeps its branch and its files

#### Scenario: A project with three repositories
- **GIVEN** the project MadSpec of De App Specialist holds appboypov/madspec, appboypov/plannotator and appboypov/omp-anytype-inbox
- **WHEN** an agent works the issue MUL-40 of MadSpec and changes appboypov/plannotator
- **THEN** its task names the three repositories
- **AND** it works in `~/Worktrees/das/plannotator/mul-40`

#### Scenario: A project without an organisation
- **GIVEN** the project das-overview has no organisation and holds appboypov/das-overview
- **WHEN** an agent works the issue MUL-7 of das-overview
- **THEN** it works in `~/Worktrees/das-overview/mul-7`

#### Scenario: A second task on the issue
- **GIVEN** an agent's task on MUL-12 left a commit in `~/Worktrees/das/multica/mul-12`
- **WHEN** Brian comments on MUL-12 and the agent takes a new task on it
- **THEN** it works in `~/Worktrees/das/multica/mul-12` with that commit

#### Scenario: A changed organisation key
- **GIVEN** `~/Worktrees/rr/rebl/mul-30` exists for the issue MUL-30 of Rebl in Rebl Records, key `rr`
- **WHEN** Brian changes Rebl Records' key to `rebl` and an agent starts on the issue MUL-31 of Rebl
- **THEN** it works in `~/Worktrees/rebl/rebl/mul-31`
- **AND** `~/Worktrees/rr/rebl/mul-30` stays where it is

### Requirement: An issue's worktree goes when the issue is done and nothing would be lost
When an issue becomes Done or Cancelled, Multica SHALL remove each of its worktrees that holds nothing uncommitted and no commit missing from its remote, and SHALL leave every other one in place and say on the issue which worktree stayed and why.

#### Scenario: A merged issue
- **GIVEN** `~/Worktrees/das/multica/mul-12` holds no uncommitted change and its commits are pushed
- **WHEN** MUL-12 becomes Done
- **THEN** `~/Worktrees/das/multica/mul-12` no longer exists

#### Scenario: Work that would be lost
- **GIVEN** `~/Worktrees/das/multica/mul-13` holds an uncommitted change
- **WHEN** MUL-13 becomes Cancelled
- **THEN** `~/Worktrees/das/multica/mul-13` stays with its change
- **AND** MUL-13 says that `~/Worktrees/das/multica/mul-13` stayed because it holds uncommitted changes
