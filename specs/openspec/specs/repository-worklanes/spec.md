# repository-worklanes Specification

## Purpose
Lets Brian see a repository's worklanes and their counts on a project's page, Projects and an organisation's page.

## Requirements


### Requirement: A repository card lists its worklanes after main
A repository card on a project's page SHALL show its number of worklanes, `main` included, next to its default branch, Add worklane in its header, and one row per worklane with its name and its state, `main` first and every other worklane after it in the order it was created. A row SHALL open its worklane's page.

#### Scenario: appboypov/rebl with three worklanes
- **GIVEN** appboypov/rebl has `main`, then Fan accounts and push, then Agenda and publication, all Free
- **WHEN** Brian opens Rebl's page
- **THEN** the card appboypov/rebl shows "3 worklanes" and Add worklane
- **AND** it lists `main`, Fan accounts and push and Agenda and publication, in that order, each Free

#### Scenario: Opening a worklane
- **WHEN** Brian clicks the row Fan accounts and push on the card appboypov/rebl
- **THEN** the page of the worklane Fan accounts and push opens

### Requirement: Projects and an organisation's page fold a repository open to its worklanes
Projects and an organisation's page SHALL let Brian fold a repository open to its worklane rows, each with its name and its state, in the order of the repository card, and SHALL show + Worklane on the repository's row. A worklane row SHALL open its worklane's page.

#### Scenario: Rebl on Projects
- **GIVEN** appboypov/rebl has `main`, Fan accounts and push and Agenda and publication
- **WHEN** Brian folds Rebl open on Projects and then appboypov/rebl
- **THEN** three worklane rows show under appboypov/rebl, `main` first
- **AND** the row appboypov/rebl shows + Worklane

#### Scenario: On the organisation's page
- **GIVEN** Rebl Records holds Rebl with appboypov/rebl and its worklane Fan accounts and push
- **WHEN** Brian folds Rebl and appboypov/rebl open on Rebl Records' page and clicks Fan accounts and push
- **THEN** the page of the worklane Fan accounts and push opens

### Requirement: Worklanes are counted on a project and its organisation
A project's page header, a project's row on Projects and on an organisation's page, an organisation's row on Projects, and an organisation's page SHALL show the number of worklanes they hold, `main` included, next to their number of repositories or projects.

#### Scenario: Rebl Records
- **GIVEN** Rebl Records holds only Rebl, which holds appboypov/rebl with `main` and two other worklanes
- **WHEN** Brian opens Projects
- **THEN** the row Rebl Records shows "1 project · 3 worklanes" and the row Rebl "1 repository · 3 worklanes"
- **AND** Rebl's page header and Rebl Records' page show "3 worklanes"

#### Scenario: A worklane created
- **GIVEN** Rebl's page header shows "3 worklanes"
- **WHEN** Brian creates the worklane Fan app on appboypov/rebl
- **THEN** Rebl's page header, the rows Rebl and Rebl Records on Projects, and Rebl Records' page show "4 worklanes"
