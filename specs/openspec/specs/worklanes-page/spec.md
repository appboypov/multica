# worklanes-page Specification

## Purpose
Lets Brian see every worklane across organisations with what it works on and its releases, and open its Timeline.

## Requirements

### Requirement: Worklanes is under Work in the sidebar after Organisations
The sidebar SHALL show Worklanes under Work directly after Organisations, and it SHALL open the Worklanes page.

#### Scenario: Opening Worklanes
- **WHEN** Brian clicks Worklanes in the sidebar
- **THEN** the Worklanes page opens and Worklanes is selected in the sidebar
- **AND** Worklanes is listed after Organisations and before Releases

### Requirement: Worklanes lists every worklane by organisation, project and repository
Worklanes SHALL show in its header the number of worklanes of the workspace, and SHALL list every worklane under its repository, the repositories under their project and the projects under their organisation, in the order Projects shows organisations and projects and a project's page shows its repositories. Each organisation SHALL show its name and its number of worklanes, each project its name, its number of repositories and its number of worklanes, each repository its `owner/name`, its default branch and + Worklane, and each repository its worklanes `main` first and the others in the order they were created.

#### Scenario: Two organisations
- **GIVEN** Rebl Records holds Rebl with appboypov/rebl and its worklanes `main`, Fan accounts and push and Agenda and publication, and De App Specialist holds Multica with appboypov/multica and its worklane `main`
- **WHEN** Brian opens Worklanes
- **THEN** the header counts 4 worklanes
- **AND** De App Specialist with "1 worklane" comes first, then Rebl Records with "3 worklanes"
- **AND** Rebl shows "1 repository · 3 worklanes", appboypov/rebl shows its branch `main` and + Worklane, and its rows are `main`, Fan accounts and push and Agenda and publication, in that order

### Requirement: Worklanes shows only what holds worklanes
Worklanes MUST NOT show an organisation, project or repository that holds no worklane, and SHALL say "No worklanes yet." when the workspace has no worklane.

#### Scenario: A project without repositories
- **GIVEN** the project LuxiLamps of Mijn Shine has no repositories and Mijn Shine holds no other project
- **WHEN** Brian opens Worklanes
- **THEN** neither Mijn Shine nor LuxiLamps shows

#### Scenario: No worklanes
- **GIVEN** no project of the workspace has a repository
- **WHEN** Brian opens Worklanes
- **THEN** it says "No worklanes yet."

### Requirement: Worklane rows on Worklanes show the worklane's releases
Each worklane row on Worklanes SHALL show its releases as a worklane row on Projects shows them: the last shipped release or "nothing shipped", the next release with "{done}/{total}" over the worklane's issues in it, and up to two planned releases after it that hold the worklane's issues; or "No releases planned" or "Does not release". The column SHALL be headed "Last · Next · Planned releases".

#### Scenario: Fan accounts and push
- **GIVEN** appboypov/rebl has shipped nothing, v1.0.0 First production release, the next, holds REBL-21 (Todo) and REBL-22 (Done) of Fan accounts and push, and v1.1.0 Account and profile holds REBL-24 of it
- **WHEN** Brian opens Worklanes
- **THEN** the column is headed "Last · Next · Planned releases"
- **AND** the row Fan accounts and push shows "nothing shipped", v1.0.0 with "1/2" and v1.1.0

#### Scenario: A repository that does not release
- **GIVEN** appboypov/rebl-site is set to "This repository does not release"
- **WHEN** Brian opens Worklanes
- **THEN** its row `main` shows "Does not release"

### Requirement: Worklanes opens the worklane's Timeline and the pages of its groups
A worklane row and each release on it SHALL open the worklane's Timeline, an organisation's name its page, a project's name its page, and + Worklane New worklane for its repository.

#### Scenario: A worklane row
- **WHEN** Brian clicks the row Fan accounts and push on Worklanes
- **THEN** the Timeline of Fan accounts and push opens

#### Scenario: An organisation and a project
- **WHEN** Brian clicks Rebl Records and, back on Worklanes, Rebl
- **THEN** Rebl Records' page opens, and then Rebl's page

#### Scenario: + Worklane
- **WHEN** Brian clicks + Worklane on the row appboypov/rebl on Worklanes
- **THEN** New worklane opens with the title "New worklane · appboypov/rebl"

### Requirement: Worklanes folds organisations, projects and repositories
Worklanes SHALL open with every organisation, project and repository open, and SHALL fold each of them closed and open again, keeping its own row.

#### Scenario: Folding an organisation
- **GIVEN** Worklanes is open with Rebl Records open
- **WHEN** Brian folds Rebl Records closed
- **THEN** Rebl and its repository and worklanes hide and Rebl Records' row stays

#### Scenario: Folding a repository
- **WHEN** Brian folds appboypov/rebl closed
- **THEN** its worklane rows hide and the row appboypov/rebl stays

### Requirement: Worklanes does not plan, edit or move releases
Worklanes MUST NOT offer Plan release, Edit release, Move earlier, Move later, Remove release or Add issue to release.

#### Scenario: No release actions
- **WHEN** Brian opens Worklanes and hovers the row Fan accounts and push and its v1.0.0
- **THEN** none of Plan release, Edit release, Move earlier, Move later, Remove release or Add issue to release shows
