## MODIFIED Requirements

### Requirement: An agent works a worklane's issue in that worklane's worktree on the issue's branch
An agent's task on an issue of a worklane SHALL run in that worklane's worktree on the issue's branch, cut from `origin/{default branch}` when it does not exist yet; a task on an issue of `main` SHALL run in the repository's checkout on the issue's branch. Once the issue has a linked open pull request for that repository, builder and verifier tasks SHALL run in its own `.pull-requests/{branch}` worktree on the pull request's branch, creating that worktree when absent and reusing it when present, and leave the lane's active branch and activity unchanged. Every task on the same issue SHALL work on the same branch.

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

#### Scenario: An earlier issue has an open pull request
- **GIVEN** REBL-21's open pull request branch is checked out in `.pull-requests/rebl-21` and the lane holds REBL-24's branch with uncommitted and unpushed work
- **WHEN** a builder or verifier takes a task on REBL-21
- **THEN** the task starts in `.pull-requests/rebl-21` on REBL-21's branch
- **AND** the lane keeps REBL-24's branch, commits, files and activity

#### Scenario: The open pull request has no worktree yet
- **GIVEN** REBL-21 has an open pull request, its pushed branch is not checked out anywhere, and `.pull-requests/rebl-21` does not exist
- **AND** the lane holds REBL-24's branch with uncommitted and unpushed work
- **WHEN** a builder or verifier takes a task on REBL-21
- **THEN** Multica creates `.pull-requests/rebl-21` on the pull request's branch at its existing branch history, not a new branch from the default branch
- **AND** the task starts there while REBL-24's branch, commits, files and activity stay unchanged

#### Scenario: The pull request branch exists only on origin
- **GIVEN** REBL-21's open pull request branch exists on origin but has no local branch or worktree
- **AND** the lane holds REBL-24's branch
- **WHEN** an agent takes a task on REBL-21
- **THEN** its PR worktree contains the pull request's pushed changes on REBL-21's branch
- **AND** the lane remains on REBL-24's branch

#### Scenario: A later run reuses review fixes
- **GIVEN** `.pull-requests/rebl-21` holds REBL-21's branch with a review-fix commit and an uncommitted file
- **WHEN** another run works REBL-21's open pull request
- **THEN** it uses that worktree with the same commit and file, without resetting or rebasing it

#### Scenario: A pull request belongs to another repository
- **GIVEN** REBL-21 belongs to a lane in appboypov/rebl and has no open pull request in that repository
- **AND** a linked open pull request belongs to another repository
- **WHEN** an agent works REBL-21 in appboypov/rebl
- **THEN** it follows the lane worktree flow for appboypov/rebl

### Requirement: A worklane runs one task at a time
A worklane SHALL run one agent task at a time in its worktree; a task using that worktree SHALL start once no other task runs there. A task on an issue with an open pull request SHALL select and lock its pull-request worktree before preparation instead of acquiring the lane's path lock, including when the pull-request worktree is absent, so the lane's active issue can run independently; tasks sharing the pull-request worktree SHALL remain mutually exclusive.

#### Scenario: Two tasks on Fan accounts and push
- **GIVEN** an agent's task on REBL-21 runs in `~/Worktrees/rebl/rebl/fan-accounts-and-push`
- **WHEN** an agent takes a task on REBL-24 of Fan accounts and push
- **THEN** the task on REBL-24 starts after the task on REBL-21 has ended

#### Scenario: Builder and verifier run at the same time
- **GIVEN** the builder runs REBL-24 in the lane worktree and holds its path lock
- **AND** REBL-21 has an open pull request in the same repository
- **WHEN** the verifier takes a task on REBL-21, both with its PR worktree already present and with it initially absent
- **THEN** the verifier starts in `.pull-requests/rebl-21` before the builder releases the lane lock
- **AND** the verifier does not enter `waiting_local_directory` for the lane
- **AND** the builder continues on the same branch with its commits, files and lane activity unchanged

#### Scenario: Two runs share one pull request worktree
- **GIVEN** a verifier holds the path lock for `.pull-requests/rebl-21`
- **WHEN** a builder takes another task on REBL-21's open pull request
- **THEN** that builder waits for `.pull-requests/rebl-21`, not the lane
- **AND** it starts only after the verifier releases the PR worktree lock
