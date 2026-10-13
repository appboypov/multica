## ADDED Requirements

### Requirement: Releases is under Work in the sidebar after Worklanes
The sidebar SHALL show Releases under Work directly after Worklanes, and it SHALL open the Releases page.

#### Scenario: Opening Releases
- **WHEN** Brian clicks Releases in the sidebar
- **THEN** the Releases page opens and Releases is selected in the sidebar
- **AND** Releases is listed after Worklanes

### Requirement: Releases lists every repository's unshipped releases by organisation and project
Releases SHALL show in its header the number of releases it lists, the columns Name, Issues and Worklanes, and every repository's unshipped releases in their order on its line, under the repository, the repositories under their project and the projects under their organisation, in the order Projects shows organisations and projects and a project's page shows its repositories. Each organisation SHALL show its name and its number of releases, each project its name, its number of repositories and its number of releases, and each repository its `owner/name` and its default branch.

#### Scenario: Rebl's line
- **GIVEN** appboypov/rebl of Rebl in Rebl Records has shipped nothing and its line holds v1.0.0 First production release, v1.0.1 Membership hardening, v1.1.0 Account and profile and v1.2.0 Notifications
- **WHEN** Brian opens Releases
- **THEN** the header counts 4 releases and the columns are Name, Issues and Worklanes
- **AND** Rebl Records shows "4 releases", Rebl "1 repository · 4 releases", and appboypov/rebl its branch `main`
- **AND** the rows under appboypov/rebl are v1.0.0, v1.0.1, v1.1.0 and v1.2.0, in that order

### Requirement: Each release row shows its version, state, title, progress and worklanes
Each release row SHALL show its version with its state, Next release for the first unshipped release on the line and Planned for the others, its title, "Set by hand" when its version was set by hand, a progress bar with "{done}/{total}" over every issue in the release, and the worklanes that hold issues in it in their order on the repository, or "No issues yet" when it holds no issue.

#### Scenario: v1.0.0 First production release
- **GIVEN** v1.0.0 First production release, the next release, holds REBL-76 (In Review) of `main` and REBL-21 (Todo) and REBL-22 (Done) of Fan accounts and push
- **WHEN** Brian opens Releases
- **THEN** its row shows v1.0.0 as Next release, First production release, "1/3", `main` and Fan accounts and push

#### Scenario: A planned release set by hand
- **GIVEN** v1.2.0 Notifications has its version set by hand and holds REBL-27 (Todo) of Fan accounts and push
- **WHEN** Brian opens Releases
- **THEN** its row shows v1.2.0 as Planned, Notifications, "Set by hand", "0/1" and Fan accounts and push

#### Scenario: A release without issues
- **GIVEN** v1.0.1 Membership hardening holds no issue
- **WHEN** Brian opens Releases
- **THEN** its row shows "No issues yet" under Worklanes

#### Scenario: A status change shows on Releases
- **GIVEN** Releases shows v1.0.0 First production release with "1/3"
- **WHEN** Brian moves REBL-21 to Done on the Board of Fan accounts and push and opens Releases again
- **THEN** v1.0.0 shows "2/3"

### Requirement: Releases shows only repositories with releases or that do not release
Releases MUST NOT show a repository that releases and has no release on its line, nor a project or organisation without a repository it shows. A repository set to "This repository does not release" SHALL show "This repository does not release" in place of its releases. Releases SHALL say "No releases yet." when it shows no repository.

#### Scenario: A repository without releases
- **GIVEN** appboypov/rebl-admin releases and its line is empty
- **WHEN** Brian opens Releases
- **THEN** appboypov/rebl-admin does not show

#### Scenario: A repository that does not release
- **GIVEN** appboypov/rebl-site of Rebl is set to "This repository does not release"
- **WHEN** Brian opens Releases
- **THEN** appboypov/rebl-site shows under Rebl with "This repository does not release"

#### Scenario: No releases
- **GIVEN** no repository of the workspace has a release and every one releases
- **WHEN** Brian opens Releases
- **THEN** it says "No releases yet."

### Requirement: A release row opens a worklane's Timeline at that release
A release row SHALL open the Timeline of the first worklane of the repository, in its order, that holds issues in the release, else of `main`, with that release in view. An organisation's name SHALL open its page and a project's name its page.

#### Scenario: v1.1.0 Account and profile
- **GIVEN** v1.1.0 Account and profile holds only REBL-24 of Fan accounts and push
- **WHEN** Brian clicks the row v1.1.0 on Releases
- **THEN** the Timeline of Fan accounts and push opens with v1.1.0 Account and profile in view

#### Scenario: Issues in two worklanes
- **GIVEN** v1.0.0 First production release holds issues of `main` and of Fan accounts and push
- **WHEN** Brian clicks the row v1.0.0 on Releases
- **THEN** the Timeline of `main` opens with v1.0.0 in view

#### Scenario: A release without issues
- **GIVEN** v1.0.1 Membership hardening holds no issue
- **WHEN** Brian clicks the row v1.0.1 on Releases
- **THEN** the Timeline of `main` of appboypov/rebl opens with v1.0.1 in view

#### Scenario: An organisation and a project
- **WHEN** Brian clicks Rebl Records and, back on Releases, Rebl
- **THEN** Rebl Records' page opens, and then Rebl's page

### Requirement: Releases folds organisations, projects and repositories
Releases SHALL open with every organisation, project and repository open, and SHALL fold each of them closed and open again, keeping its own row.

#### Scenario: Folding a project
- **GIVEN** Releases is open with Rebl open
- **WHEN** Brian folds Rebl closed
- **THEN** appboypov/rebl and its releases hide and Rebl's row stays

#### Scenario: Folding a repository
- **WHEN** Brian folds appboypov/rebl closed and open again
- **THEN** its release rows hide and then show again

### Requirement: Releases does not plan, edit or move releases
Releases MUST NOT offer Plan release, Edit release, Move earlier, Move later, Remove release or Add issue to release.

#### Scenario: No release actions
- **WHEN** Brian opens Releases and hovers the row v1.1.0 Account and profile
- **THEN** none of Plan release, Edit release, Move earlier, Move later, Remove release or Add issue to release shows
