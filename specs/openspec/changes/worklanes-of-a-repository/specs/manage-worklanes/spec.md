## ADDED Requirements

### Requirement: Rename changes a worklane's name, not its folder
A worklane's menu, on its row and on its page, SHALL offer Rename for every worklane but `main`, which SHALL save a new name only when it is unique in the repository, `main` included and ignoring case, and SHALL keep the worklane's folder and worktree.

#### Scenario: Renaming Fan app
- **GIVEN** the worklane Fan app of appboypov/rebl works in `~/Worktrees/rebl/rebl/fan-app`
- **WHEN** Brian renames it to "Fan accounts"
- **THEN** the card appboypov/rebl lists Fan accounts in Fan app's place
- **AND** Fan accounts still works in `~/Worktrees/rebl/rebl/fan-app`

#### Scenario: A taken name
- **GIVEN** appboypov/rebl has the worklanes Fan app and Agenda and publication
- **WHEN** Brian renames Fan app to "agenda and publication"
- **THEN** Rename says appboypov/rebl already has a worklane agenda and publication and saves nothing

### Requirement: Remove removes a worklane and its worktree only when nothing would be lost
A worklane's menu SHALL offer Remove for every worklane but `main`. Remove SHALL be refused, with the reason, while the worklane's runtime is offline or its worktree holds an uncommitted change or a commit missing from `origin/{default branch}`. Otherwise it SHALL remove the worktree on the runtime and the worklane from Multica.

#### Scenario: A clean worklane
- **GIVEN** the worklane Fan app of appboypov/rebl has no issues and `~/Worktrees/rebl/rebl/fan-app` holds nothing uncommitted and no commit missing from origin/main
- **WHEN** Brian removes Fan app
- **THEN** the card appboypov/rebl no longer lists Fan app and Rebl's counts show one worklane fewer
- **AND** `~/Worktrees/rebl/rebl/fan-app` no longer exists on Brian's Mac

#### Scenario: Uncommitted work
- **GIVEN** `~/Worktrees/rebl/rebl/fan-app` holds an uncommitted change
- **WHEN** Brian removes the worklane Fan app
- **THEN** Remove says Fan app's worktree holds uncommitted changes and removes nothing
- **AND** `~/Worktrees/rebl/rebl/fan-app` keeps its change

#### Scenario: Unmerged commits
- **GIVEN** `~/Worktrees/rebl/rebl/fan-app` holds a commit that origin/main does not have
- **WHEN** Brian removes the worklane Fan app
- **THEN** Remove says Fan app's worktree holds commits that are not on origin/main and removes nothing

#### Scenario: The runtime is offline
- **GIVEN** Brian's Mac, the runtime of Fan app, is offline
- **WHEN** Brian removes the worklane Fan app
- **THEN** Remove says Brian's Mac is offline and removes nothing

#### Scenario: main
- **WHEN** Brian opens the menu of the worklane `main` of appboypov/rebl
- **THEN** it offers neither Rename nor Remove

### Requirement: A repository with a worklane besides main cannot be removed
Remove repository SHALL be refused, naming the worklanes, while the repository has a worklane besides `main`.

#### Scenario: appboypov/rebl with Fan app
- **GIVEN** appboypov/rebl has the worklanes `main` and Fan app
- **WHEN** Brian chooses Remove repository on the card appboypov/rebl
- **THEN** it says appboypov/rebl still has the worklane Fan app and removes nothing

#### Scenario: After the worklane is removed
- **GIVEN** Brian removed the worklane Fan app, the only worklane of appboypov/rebl besides `main`, and appboypov/rebl has no issues and no releases
- **WHEN** he chooses Remove repository on the card appboypov/rebl
- **THEN** appboypov/rebl is removed from Rebl
