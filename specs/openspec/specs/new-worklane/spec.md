# new-worklane Specification

## Purpose
Lets Brian create a worklane on a repository, with its worktree made on the runtime step by step.

## Requirements


### Requirement: Add worklane opens New worklane
Add worklane on a repository's card on a project's page, and + Worklane on a repository's row on Projects and on an organisation's page, SHALL open New worklane with the title "New worklane · {owner/name}", with the fields Name, Folder, Runtime and Worktree and the buttons Cancel and Create worklane.

#### Scenario: From the repository card
- **GIVEN** the project Rebl holds appboypov/rebl
- **WHEN** Brian clicks Add worklane on the card appboypov/rebl
- **THEN** New worklane opens with the title "New worklane · appboypov/rebl"

#### Scenario: From Projects
- **GIVEN** Rebl is folded open to appboypov/rebl on Projects
- **WHEN** Brian clicks + Worklane on the row appboypov/rebl
- **THEN** New worklane opens with the title "New worklane · appboypov/rebl"

### Requirement: A worklane's name is unique in its repository
Name SHALL be required, and a name that a worklane of the same repository already has, `main` included and ignoring case, SHALL show under Name as "{owner/name} already has a worklane {name}" and keep Create worklane disabled.

#### Scenario: A taken name
- **GIVEN** appboypov/rebl has the worklanes `main` and Fan accounts and push
- **WHEN** Brian types "fan accounts and push" as Name
- **THEN** Name says "appboypov/rebl already has a worklane fan accounts and push"
- **AND** Create worklane is disabled

#### Scenario: main is taken
- **WHEN** Brian types "Main" as Name in New worklane for appboypov/rebl
- **THEN** Name says appboypov/rebl already has a worklane Main and Create worklane is disabled

#### Scenario: The same name in another repository
- **GIVEN** appboypov/rebl-admin has a worklane Fan app
- **WHEN** Brian types "Fan app" as Name in New worklane for appboypov/rebl
- **THEN** Name shows no error

### Requirement: The folder fills in from the name and is unique among the repository's worklanes
Folder SHALL fill in from Name in lowercase, each run of other characters a dash, without a dash at the start or end, until Brian edits it. A folder SHALL be one name of lowercase letters, digits and dashes starting with a letter or a digit, and unique among the repository's worklanes; Folder SHALL say which rule a folder breaks, and Create worklane SHALL stay disabled while it breaks one.

#### Scenario: Filled in
- **WHEN** Brian types "Fan app" as Name
- **THEN** Folder shows `fan-app`
- **AND** under Folder "Unique among appboypov/rebl's worklanes"

#### Scenario: Edited
- **GIVEN** Brian typed "Fan app" as Name
- **WHEN** he changes Folder to `fans` and then Name to "Fan app v2"
- **THEN** Folder still shows `fans`

#### Scenario: Not one folder name
- **WHEN** Brian changes Folder to `Fan/App`
- **THEN** Folder says "One folder name: lowercase letters, digits and dashes"
- **AND** Create worklane is disabled

#### Scenario: A folder in use
- **GIVEN** the worklane Fan accounts and push of appboypov/rebl has the folder `fan-accounts-and-push`
- **WHEN** Brian changes Folder to `fan-accounts-and-push`
- **THEN** Folder says "Already used by Fan accounts and push"
- **AND** Create worklane is disabled

### Requirement: Runtime offers only the online runtime that holds the repository's checkout
Runtime SHALL list the workspace's runtimes, each with online or offline, and SHALL let Brian pick only the runtime that holds the repository's checkout, while it is online and can make worktrees; every other runtime SHALL show why it cannot be picked. Runtime SHALL be filled in with that runtime when it can be picked, and Create worklane SHALL stay disabled while no runtime is picked.

#### Scenario: Brian's Mac holds the checkout
- **GIVEN** appboypov/rebl's checkout is `~/Repos/rebl` on Brian's Mac, which is online
- **WHEN** New worklane opens for appboypov/rebl
- **THEN** Runtime shows "Brian's Mac · online"
- **AND** under Runtime "Online runtimes that can make worktrees"

#### Scenario: A runtime without the checkout
- **GIVEN** the runtime Studio Mac mini is online and holds no checkout of appboypov/rebl
- **WHEN** Brian opens the Runtime list in New worklane for appboypov/rebl
- **THEN** Studio Mac mini shows "No checkout of appboypov/rebl" and cannot be picked

#### Scenario: The checkout's runtime is offline
- **GIVEN** Brian's Mac, which holds appboypov/rebl's checkout, is offline
- **WHEN** New worklane opens for appboypov/rebl
- **THEN** Runtime shows Brian's Mac as offline and picks no runtime
- **AND** Create worklane is disabled

#### Scenario: A runtime that cannot make worktrees
- **GIVEN** the runtime that holds appboypov/rebl's checkout is online with a daemon that cannot make worktrees
- **WHEN** Brian opens the Runtime list in New worklane for appboypov/rebl
- **THEN** that runtime shows that it cannot make worktrees and cannot be picked

### Requirement: Worktree shows the path, its base and the install command before creating
Worktree SHALL show, read only and following the fields above it, the worktree's path `~/Worktrees/{organisation key}/{repo}/{folder}` with `{repo}` the repository's name, or `~/Worktrees/{repo}/{folder}` for a project without an organisation, "from origin/{default branch}", and the repository's install command, or that it installs nothing when the install command is None.

#### Scenario: Fan app in Rebl
- **GIVEN** Rebl belongs to Rebl Records with key `rebl`, and appboypov/rebl has default branch main and install command `pnpm install`
- **WHEN** Brian names the worklane "Fan app"
- **THEN** Worktree shows `~/Worktrees/rebl/rebl/fan-app`, "from origin/main" and `pnpm install`

#### Scenario: A project without an organisation
- **GIVEN** the project das-overview has no organisation and holds appboypov/das-overview
- **WHEN** Brian names a worklane "Docs" in New worklane for appboypov/das-overview
- **THEN** Worktree shows `~/Worktrees/das-overview/docs`

#### Scenario: No install command
- **GIVEN** appboypov/das-overview's install command is None
- **WHEN** New worklane opens for appboypov/das-overview
- **THEN** Worktree says it installs nothing

### Requirement: Create worklane makes the worktree on the runtime step by step
Create worklane SHALL be enabled only while Name, Folder and Runtime are valid. Clicking it SHALL replace the fields with the worklane's name, its worktree path and "Creating on {runtime}" with the steps "Fetch origin/{default branch}", "Add worktree" with its path, "Install dependencies" with the install command, and "Open the worklane", each shown as pending, running or done as it runs on the runtime, with the time a finished fetch took. While it runs, the button SHALL read "Creating worklane", and Cancel, the button and closing the dialog SHALL be disabled. Add worktree SHALL make the worktree at the commit of `origin/{default branch}` just fetched. Install dependencies SHALL run the install command in the worktree, and SHALL show as skipped when the install command is None.

#### Scenario: Fan app is created
- **GIVEN** New worklane for appboypov/rebl shows Name "Fan app", Folder `fan-app` and Runtime "Brian's Mac · online"
- **WHEN** Brian clicks Create worklane
- **THEN** the dialog shows "Creating on Brian's Mac" with Fetch origin/main, Add worktree `~/Worktrees/rebl/rebl/fan-app`, Install dependencies `pnpm install` and Open the worklane
- **AND** each step shows as running while it runs and as done after it
- **AND** Cancel is disabled until the steps end
- **AND** afterwards `~/Worktrees/rebl/rebl/fan-app` on Brian's Mac is a worktree of `~/Repos/rebl` at the commit of origin/main, with the dependencies `pnpm install` installed

#### Scenario: The normal checkout is untouched
- **GIVEN** `~/Repos/rebl` is on main with an uncommitted change
- **WHEN** Brian creates the worklane Fan app on appboypov/rebl
- **THEN** `~/Repos/rebl` is still on main with its uncommitted change

#### Scenario: Nothing to install
- **GIVEN** appboypov/das-overview's install command is None
- **WHEN** Brian creates the worklane Docs on appboypov/das-overview
- **THEN** Install dependencies shows as skipped and the worklane is created

### Requirement: A folder already at the worktree's path is used
When the worktree's path already holds a git checkout whose `origin` is the repository, Add worktree SHALL use it as the worklane's worktree as it is, with its branch and its files, and say so. When the path is an empty folder, Add worktree SHALL make the worktree in it. When the path holds anything else, Add worktree SHALL fail with the reason and how to fix it.

#### Scenario: A worktree of the repository is there
- **GIVEN** `~/Worktrees/rebl/rebl/fan-app` is a worktree of `~/Repos/rebl` on the branch fan-app/profile with an uncommitted change
- **WHEN** Brian creates the worklane Fan app on appboypov/rebl
- **THEN** Add worktree says it uses the worktree that is there
- **AND** the worklane Fan app is created with `~/Worktrees/rebl/rebl/fan-app` as its worktree, still on fan-app/profile with its uncommitted change

#### Scenario: Something else is there
- **GIVEN** `~/Worktrees/rebl/rebl/fan-app` holds files and no git checkout of appboypov/rebl
- **WHEN** Brian creates the worklane Fan app on appboypov/rebl
- **THEN** Add worktree fails and says `~/Worktrees/rebl/rebl/fan-app` holds something that is not a checkout of appboypov/rebl, to move it or pick another folder
- **AND** the files in `~/Worktrees/rebl/rebl/fan-app` are unchanged

### Requirement: A failed step keeps its error with Try again and saves no worklane
When a step fails, the dialog SHALL show that step as failed with what failed, the error's output and how to fix it, with "No worklane is saved until every step passes.", the steps after it as pending, and the buttons Cancel and Try again. Try again SHALL run the failed step and the steps after it again. No worklane SHALL exist in Multica until every step passed. Cancel SHALL close the dialog and leave what the steps made on the runtime in place.

#### Scenario: Install fails
- **GIVEN** appboypov/rebl's `pnpm-lock.yaml` on main is not up to date with its `package.json`
- **WHEN** Brian creates the worklane Fan app on appboypov/rebl
- **THEN** Fetch origin/main and Add worktree show as done, Install dependencies as failed with "pnpm install exited with 1", and Open the worklane as pending
- **AND** the dialog shows the error ERR_PNPM_OUTDATED_LOCKFILE with "No worklane is saved until every step passes." and the buttons Cancel and Try again
- **AND** appboypov/rebl shows no worklane Fan app on Rebl's page

#### Scenario: Try again after the fix
- **GIVEN** Install dependencies failed for Fan app and Brian fixed the lockfile on main
- **WHEN** he clicks Try again
- **THEN** Install dependencies runs again and, once it passes, the worklane Fan app opens

#### Scenario: Fetch fails
- **GIVEN** Brian's Mac cannot reach GitHub
- **WHEN** Brian creates the worklane Fan app on appboypov/rebl
- **THEN** Fetch origin/main fails with the error and Try again
- **AND** no folder `~/Worktrees/rebl/rebl/fan-app` was made

#### Scenario: Cancel after a failure
- **GIVEN** Install dependencies failed for Fan app after Add worktree made `~/Worktrees/rebl/rebl/fan-app`
- **WHEN** Brian clicks Cancel
- **THEN** the dialog closes and appboypov/rebl has no worklane Fan app
- **AND** `~/Worktrees/rebl/rebl/fan-app` is still on Brian's Mac
- **AND** creating Fan app again with the folder `fan-app` uses it

### Requirement: When every step passes the worklane opens, Free
When every step passed, Multica SHALL save the worklane with its name, folder, runtime and worktree, after the repository's other worklanes, and open its page on its timeline with the state Free.

#### Scenario: Fan app opens
- **GIVEN** appboypov/rebl has the worklanes `main` and Fan accounts and push
- **WHEN** every step of creating Fan app passes
- **THEN** the page of the worklane Fan app opens on its timeline with the state Free
- **AND** the card appboypov/rebl on Rebl's page lists `main`, Fan accounts and push and Fan app, in that order

#### Scenario: A changed organisation key
- **GIVEN** the worklane Fan app of appboypov/rebl in Rebl Records, key `rebl`, works in `~/Worktrees/rebl/rebl/fan-app`
- **WHEN** Brian changes Rebl Records' key to `rr` and creates the worklane Agenda on appboypov/rebl
- **THEN** Agenda's worktree is `~/Worktrees/rr/rebl/agenda`
- **AND** Fan app still works in `~/Worktrees/rebl/rebl/fan-app`
