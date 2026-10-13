## ADDED Requirements

### Requirement: A project's page shows one card per repository
A project's page SHALL show its number of repositories in its header and one card per repository with its `owner/name`, its default branch and its worklane `main` with its state, in place of Resources. A card SHALL fold open and closed.

#### Scenario: Rebl with one repository
- **GIVEN** the project Rebl holds the repository appboypov/rebl with default branch main
- **WHEN** Brian opens Rebl's page
- **THEN** its header shows "1 repository"
- **AND** the card appboypov/rebl shows "main" as its default branch and the worklane `main`, Free

#### Scenario: A project without repositories
- **GIVEN** the project DAS HQ holds no repository
- **WHEN** Brian opens DAS HQ's page
- **THEN** its header shows "0 repositories" and its page shows no card

#### Scenario: A card folds
- **GIVEN** the card appboypov/rebl is open on Rebl's page
- **WHEN** Brian clicks its header
- **THEN** its worklane `main` hides and the card's header stays

### Requirement: No project has resources
No page, dialog or menu SHALL show a project's resources or attach a GitHub repository or a local folder to a project other than as a repository.

#### Scenario: A project's page
- **WHEN** Brian opens a project's page
- **THEN** it shows no Resources section and offers no way to add a GitHub link or a local folder

### Requirement: Projects and an organisation's page fold a project open to its repositories
Projects and an organisation's page SHALL let Brian fold a project open to one row per repository, each with its `owner/name` and default branch.

#### Scenario: MadSpec's repositories on Projects
- **GIVEN** the project MadSpec holds appboypov/madspec, appboypov/plannotator and appboypov/omp-anytype-inbox
- **WHEN** Brian folds MadSpec open on Projects
- **THEN** three rows show under MadSpec, each with its `owner/name` and default branch

#### Scenario: On the organisation's page
- **GIVEN** Rebl Records holds Rebl with appboypov/rebl
- **WHEN** Brian folds Rebl open on Rebl Records' page
- **THEN** the row appboypov/rebl shows under Rebl

### Requirement: Organisations and an organisation's page count repositories
Organisations SHALL show each organisation's number of repositories next to its number of projects, and an organisation's page SHALL show its number of repositories next to its number of projects; the number is the sum over the organisation's projects.

#### Scenario: De App Specialist
- **GIVEN** De App Specialist holds MadSpec with 3 repositories, Multica with 1 and DAS HQ with none
- **WHEN** Brian opens Organisations
- **THEN** De App Specialist shows "3 projects" and "4 repositories"
- **AND** De App Specialist's page shows "3 projects" and "4 repositories"

#### Scenario: A repository added
- **GIVEN** Rebl Records shows "1 repository" on Organisations
- **WHEN** Brian adds appboypov/rebl-admin to Rebl
- **THEN** Rebl Records shows "2 repositories" on Organisations and on its page

### Requirement: Edit repository changes the checkout and the releases
A repository card's menu SHALL offer Edit repository, which SHALL show the repository's checkout and releases as Add repository does, filled in with what the repository holds, apply the same checks, and save only a checkout that passes them. It MUST NOT change which GitHub repository the repository is.

#### Scenario: A moved checkout
- **GIVEN** Brian moved the checkout of appboypov/rebl to `~/Code/rebl`
- **WHEN** he chooses `~/Code/rebl` in Edit repository and saves
- **THEN** the repository's checkout is `~/Code/rebl` and Edit repository shows it the next time it opens

#### Scenario: A failed check
- **GIVEN** `~/Code/notes` is a folder without git
- **WHEN** Brian chooses `~/Code/notes` in Edit repository
- **THEN** Checkout says the folder is not a git checkout and Save is disabled

#### Scenario: Stop releasing
- **WHEN** Brian picks "This repository does not release" in Edit repository and saves
- **THEN** Edit repository shows "This repository does not release" the next time it opens

#### Scenario: The repository is fixed
- **WHEN** Brian opens Edit repository for appboypov/rebl
- **THEN** it shows appboypov/rebl and offers no way to pick another GitHub repository

### Requirement: Remove repository removes it from Multica only
A repository card's menu SHALL offer Remove repository, which SHALL remove the repository and its worklane `main` from the project. It MUST NOT delete or change the checkout folder.

#### Scenario: Removing appboypov/rebl-admin
- **GIVEN** Rebl holds appboypov/rebl-admin with only the worklane `main` and its checkout `~/Repos/rebl-admin`
- **WHEN** Brian removes appboypov/rebl-admin
- **THEN** Rebl's page shows no card appboypov/rebl-admin and its header counts one repository fewer
- **AND** `~/Repos/rebl-admin` is still on Brian's Mac, unchanged
- **AND** Add repository on any project offers appboypov/rebl-admin again
