## ADDED Requirements

### Requirement: Brian turns Work in the project folder on and off
Edit repository and `multica repo update <owner/name> --work-in-project-folder=<true|false>` SHALL let Brian turn Work in the project folder on and off for a repository, the repository SHALL keep what he saved, and every open view of the workspace SHALL show the change without a reload. A repository SHALL have the setting off until Brian turns it on.

#### Scenario: Turned on in the app
- **GIVEN** appboypov/multica has Work in the project folder off
- **WHEN** Brian turns it on in Edit repository for appboypov/multica and saves
- **THEN** Edit repository for appboypov/multica shows Work in the project folder on
- **AND** another open window of the workspace shows it on without a reload

#### Scenario: Turned off with the CLI
- **GIVEN** appboypov/multica has Work in the project folder on
- **WHEN** Brian runs `multica repo update appboypov/multica --work-in-project-folder=false`
- **THEN** Edit repository for appboypov/multica shows Work in the project folder off

#### Scenario: A repository from before
- **GIVEN** appboypov/rebl was added before repositories had this setting
- **WHEN** Brian opens Edit repository for appboypov/rebl
- **THEN** Work in the project folder shows off

#### Scenario: An unknown repository in the CLI
- **WHEN** Brian runs `multica repo update appboypov/nope --work-in-project-folder=true` and no project of the workspace holds appboypov/nope
- **THEN** the command fails and says that appboypov/nope is not found in this workspace

### Requirement: An issue task of such a repository works in its checkout
With Work in the project folder on, an agent's task on an issue of the repository, on the machine of the repository's checkout, SHALL work in that checkout on the issue's branch, cut from `origin/{default branch}` when it does not exist yet, or on the branch of the issue's open pull request in that repository, whichever worklane the issue is in. It MUST NOT make an issue worktree or a pull-request worktree, and MUST NOT work in a worklane's worktree. The issue SHALL keep its worklane and its place in that worklane's order, and the checkout's `main` worklane SHALL show the branch the task switched to. An issue whose branch the checkout still holds SHALL go on in the checkout after the setting is turned off.

#### Scenario: An issue in no worklane
- **GIVEN** appboypov/multica has Work in the project folder on and its checkout `~/Repos/Forks/multica` on Brian's Mac
- **WHEN** an agent on Brian's Mac takes a task on WORK-1400 of Multica, which has no worklane
- **THEN** it works in `~/Repos/Forks/multica` on WORK-1400's branch
- **AND** no folder for WORK-1400 appears under `~/Worktrees`
- **AND** `main` of appboypov/multica shows WORK-1400's branch

#### Scenario: An issue with an open pull request
- **GIVEN** appboypov/multica has Work in the project folder on
- **AND** WORK-1400 has an open pull request in appboypov/multica on the branch `work-1400`
- **WHEN** a verifier takes a task on WORK-1400
- **THEN** it works in `~/Repos/Forks/multica` on `work-1400`
- **AND** no folder under `~/Worktrees` holds `work-1400`

#### Scenario: A pull request of another repository
- **GIVEN** appboypov/multica has Work in the project folder on
- **AND** WORK-1400's only open pull request belongs to appboypov/madspec
- **WHEN** an agent takes a task on WORK-1400
- **THEN** it works in `~/Repos/Forks/multica` on WORK-1400's own branch

#### Scenario: The setting off
- **GIVEN** appboypov/rebl has Work in the project folder off and its checkout `~/Repos/rebl`
- **WHEN** an agent takes a task on REBL-30 of Rebl, which has no worklane
- **THEN** it works in `~/Worktrees/rebl/rebl/rebl-30`

#### Scenario: An issue of another worklane
- **GIVEN** appboypov/rebl has Work in the project folder on and its checkout `~/Repos/rebl`
- **AND** its worklane Fan accounts and push has its worktree `~/Worktrees/rebl/rebl/fan-accounts-and-push`
- **WHEN** an agent takes a task on REBL-21 of Fan accounts and push
- **THEN** it works in `~/Repos/rebl` on REBL-21's branch
- **AND** REBL-21 stays in Fan accounts and push at its place in the order
- **AND** `~/Worktrees/rebl/rebl/fan-accounts-and-push` keeps its branch and files

#### Scenario: The setting turned off while the checkout holds the issue's branch
- **GIVEN** `~/Repos/Forks/multica` is on `work-1400` from a task with Work in the project folder on
- **AND** Brian turned Work in the project folder off for appboypov/multica
- **WHEN** an agent takes a task on WORK-1400
- **THEN** it works in `~/Repos/Forks/multica` on `work-1400`

### Requirement: Tasks in one checkout run one after the other
Tasks that work in the same checkout SHALL run one at a time: a task SHALL start there once no other task works there, and SHALL show that it waits for the checkout meanwhile.

#### Scenario: Two tasks start together
- **GIVEN** appboypov/multica has Work in the project folder on
- **WHEN** agents take a task on WORK-1400 and a task on WORK-1401 of Multica at the same moment
- **THEN** one task starts in `~/Repos/Forks/multica`
- **AND** the other shows that it waits for `multica` and starts after the first has ended

### Requirement: A task never leaves work behind on the checkout's branch
A task that works in a checkout SHALL switch it to its branch only when the checkout's current branch holds no uncommitted change and no commit missing from its remote; otherwise it MUST fail without switching, with a reason that names the branch and says whether uncommitted changes or the number of unpushed commits held it.

#### Scenario: Uncommitted changes
- **GIVEN** `~/Repos/Forks/multica` is on `work-1400` with an uncommitted change
- **WHEN** an agent takes a task on WORK-1401 of Multica
- **THEN** the task fails with the reason that `main` holds uncommitted changes on `work-1400`
- **AND** `~/Repos/Forks/multica` stays on `work-1400` with its change

#### Scenario: Unpushed commits
- **GIVEN** `~/Repos/Forks/multica` is on `work-1400` with two commits that are not pushed
- **WHEN** an agent takes a task on WORK-1401 of Multica
- **THEN** the task fails with the reason that `main` holds 2 commits on `work-1400` that are not pushed
