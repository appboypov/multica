## ADDED Requirements

### Requirement: Each repository of a Product is a repository of its project
The brain sync SHALL add each Repository object a Product links to that Product's project as a repository with its checkout on the sync's runtime, the version policy Add repository would pick first, and the worklane `main` on its checkout, Free, applying Add repository's checks. Once a repository is added, the project MUST NOT keep the local folder resource the sync made for its checkout.

#### Scenario: MadSpec's repositories
- **GIVEN** the Product MadSpec links appboypov/madspec, appboypov/plannotator and appboypov/omp-anytype-inbox, each with one checkout under `~/Repos` on its default branch
- **AND** the projects madspec, plannotator and omp-anytype-inbox, which the sync kept for MadSpec, each hold a local folder resource for one of those checkouts
- **WHEN** the sync runs
- **THEN** MadSpec's page shows the three repositories as cards, each with worklane `main` Free
- **AND** no project holds a local folder resource for those checkouts

#### Scenario: The version policy
- **GIVEN** appboypov/rebl has `pubspec.yaml` and `package.json`
- **WHEN** the sync adds appboypov/rebl
- **THEN** Edit repository for appboypov/rebl shows "Version in pubspec.yaml · tags v{version}"

#### Scenario: A second run
- **GIVEN** a run added every repository
- **WHEN** the sync runs again without a change in the brain or on disk
- **THEN** it writes nothing

### Requirement: A repository the sync cannot add is reported and left alone
The brain sync SHALL leave out and print on every run each repository it cannot add, with the reason: the GitHub App cannot see it, it has no checkout or more than one, its checkout fails a check, or it is already a repository of another project. It SHALL add the repository on the first run where none of these holds.

#### Scenario: Not on the default branch
- **GIVEN** `~/Repos/rebl` is a git checkout of appboypov/rebl on feature/login
- **WHEN** the sync runs
- **THEN** it prints that appboypov/rebl's checkout is on feature/login, not main, and adds no repository appboypov/rebl
- **AND** after Brian switches `~/Repos/rebl` to main and the sync runs again, Rebl shows appboypov/rebl

#### Scenario: The GitHub App cannot see it
- **GIVEN** the GitHub App cannot see appboypov/fan-widgets
- **WHEN** the sync runs
- **THEN** it prints that the GitHub App cannot see appboypov/fan-widgets and adds no repository appboypov/fan-widgets

#### Scenario: In another project Brian chose
- **GIVEN** Brian added appboypov/rebl-admin to the project Rebl
- **AND** the brain links appboypov/rebl-admin to the Product Rebl Admin
- **WHEN** the sync runs
- **THEN** appboypov/rebl-admin stays in Rebl, and no issue moves out of Rebl
- **AND** the sync prints that appboypov/rebl-admin is in Rebl

### Requirement: What Brian set on a repository stays
The brain sync MUST NOT change a repository's checkout or version policy once the repository exists in Multica.

#### Scenario: An edited checkout
- **GIVEN** Brian set appboypov/rebl's checkout to `~/Code/rebl` with Edit repository
- **AND** the brain's Repository appboypov/rebl has its checkout at `~/Repos/rebl`
- **WHEN** the sync runs
- **THEN** appboypov/rebl's checkout stays `~/Code/rebl`

#### Scenario: An edited version policy
- **GIVEN** Brian set appboypov/rebl to "This repository does not release"
- **WHEN** the sync runs
- **THEN** appboypov/rebl stays "This repository does not release"

### Requirement: The brain decides which repositories a project holds
While the brain links a Repository to a Product, the brain sync SHALL add it again to the Product's project after Brian removed it in Multica. When a Repository object is deleted from the brain, the sync SHALL leave its repository in Multica as it is and SHALL print it on every run.

#### Scenario: Removed in Multica, linked in the brain
- **GIVEN** Brian removed appboypov/plannotator from MadSpec
- **AND** the brain's Product MadSpec still links appboypov/plannotator
- **WHEN** the sync runs
- **THEN** MadSpec shows appboypov/plannotator again with worklane `main`

#### Scenario: Deleted from the brain
- **GIVEN** MadSpec holds the synced repository appboypov/omp-anytype-inbox
- **WHEN** its Repository object is deleted from the brain and the sync runs
- **THEN** MadSpec still shows appboypov/omp-anytype-inbox
- **AND** the sync prints that appboypov/omp-anytype-inbox has no Repository in the brain

### Requirement: Kept repository projects merge into their Product's project
For each project the sync kept for a repository of a Product with several repositories, the brain sync SHALL move the project's issues into the Product's project and SHALL then delete the emptied project, once that repository is a repository of the Product's project. While it is not, the kept project MUST stay with its issues.

#### Scenario: MadSpec takes in its repositories' issues
- **GIVEN** the sync kept the projects madspec, plannotator and omp-anytype-inbox, with 10, 2 and 1 issues, for the Product MadSpec
- **WHEN** the sync runs and adds appboypov/madspec, appboypov/plannotator and appboypov/omp-anytype-inbox to MadSpec
- **THEN** the project MadSpec holds those 13 issues
- **AND** the projects madspec, plannotator and omp-anytype-inbox no longer exist

#### Scenario: A repository the sync cannot add keeps its project
- **GIVEN** the sync kept the project sm-apify with its issues for the Product SuperMarty
- **AND** the GitHub App cannot see Supermarty-B-V/sm-apify
- **WHEN** the sync runs
- **THEN** it prints that the GitHub App cannot see Supermarty-B-V/sm-apify
- **AND** the project sm-apify stays with its issues
