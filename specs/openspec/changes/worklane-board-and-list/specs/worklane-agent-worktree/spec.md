## ADDED Requirements

### Requirement: An agent works a worklane's issue in that worklane's worktree on the issue's branch
An agent's task on an issue of a worklane SHALL run in that worklane's worktree on the issue's branch, cut from `origin/{default branch}` when it does not exist yet; a task on an issue of `main` SHALL run in the repository's checkout on the issue's branch. Every task on the same issue SHALL work on the same branch.

#### Scenario: REBL-21 in Fan accounts and push
- **GIVEN** Fan accounts and push has its worktree `~/Worktrees/rebl/rebl/fan-accounts-and-push` on Brian's Mac and is Free
- **WHEN** an agent on Brian's Mac takes a task on REBL-21 of Fan accounts and push
- **THEN** it works in `~/Worktrees/rebl/rebl/fan-accounts-and-push` on REBL-21's branch, cut from `origin/main`
- **AND** Fan accounts and push shows Busy with "now: REBL-21 A fan changes their display name"

#### Scenario: An issue of main
- **GIVEN** appboypov/rebl has its checkout `~/Repos/rebl` on Brian's Mac
- **WHEN** an agent takes a task on REBL-30 of `main`
- **THEN** it works in `~/Repos/rebl` on REBL-30's branch

#### Scenario: A second task on the issue
- **GIVEN** an agent's task on REBL-21 left a commit on REBL-21's branch
- **WHEN** Brian comments on REBL-21 and the agent takes a new task on it
- **THEN** it works in `~/Worktrees/rebl/rebl/fan-accounts-and-push` on REBL-21's branch with that commit

### Requirement: A worklane runs one task at a time
A worklane SHALL run one agent task at a time in its worktree; a task on one of its issues SHALL start once no other task runs there.

#### Scenario: Two tasks on Fan accounts and push
- **GIVEN** an agent's task on REBL-21 runs in `~/Worktrees/rebl/rebl/fan-accounts-and-push`
- **WHEN** an agent takes a task on REBL-24 of Fan accounts and push
- **THEN** the task on REBL-24 starts after the task on REBL-21 has ended

### Requirement: A task from an agent on another runtime is refused with the reason
A task on an issue of a worklane from an agent whose runtime is not the worklane's runtime MUST NOT start, and the issue SHALL say "{agent} runs on {agent's runtime}; {worklane} lives on {worklane's runtime}".

#### Scenario: An agent on Studio Mac mini
- **GIVEN** Fan accounts and push lives on Brian's Mac and the agent Builder runs on Studio Mac mini
- **WHEN** Builder takes a task on REBL-24 of Fan accounts and push
- **THEN** the task does not start
- **AND** REBL-24 says "Builder runs on Studio Mac mini; Fan accounts and push lives on Brian's Mac"

### Requirement: After a merge the worktree switches to the next issue's branch when nothing would be lost
When the pull request of a worklane's active issue merges into the default branch, Multica SHALL fetch the default branch and switch the worklane's worktree to the branch of the issue it takes next, cut from the fetched default branch when that branch does not exist yet, only while the worktree holds nothing uncommitted and no commit missing from its remote; otherwise the worktree SHALL keep its branch and files and the worklane's activity SHALL say why.

#### Scenario: REBL-21 merges
- **GIVEN** Fan accounts and push is on REBL-21's branch with nothing uncommitted or unpushed, and takes REBL-24 next
- **WHEN** REBL-21's pull request merges into `main`
- **THEN** `~/Worktrees/rebl/rebl/fan-accounts-and-push` is on REBL-24's branch, cut from the fetched `origin/main`

#### Scenario: Work that would be lost
- **GIVEN** `~/Worktrees/rebl/rebl/fan-accounts-and-push` holds an uncommitted change on REBL-21's branch
- **WHEN** REBL-21's pull request merges into `main`
- **THEN** the worktree stays on REBL-21's branch with its change
- **AND** the activity of Fan accounts and push says the worktree stayed on REBL-21's branch because it holds uncommitted changes
