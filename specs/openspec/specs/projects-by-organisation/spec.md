# projects-by-organisation Specification

## Purpose

Every project belongs to at most one organisation. Projects groups projects by organisation with No organisation last, New project asks for a name and an organisation, a project's page changes its organisation, and no page, dialog or filter shows a project status, start date or due date.

## Requirements

### Requirement: Projects groups projects by organisation
Projects SHALL list every project of the workspace under its organisation, the organisations by name and then "No organisation" with the projects that have none. Each organisation's group SHALL show its name and number of projects, fold open and closed, and open the organisation's page from its name. An organisation without projects MUST NOT show on Projects.

#### Scenario: Projects by organisation
- **GIVEN** Rebl Records holds Rebl, De App Specialist holds Multica, and the project das-overview has no organisation
- **WHEN** Brian opens Projects
- **THEN** De App Specialist with Multica comes first, then Rebl Records with Rebl, then "No organisation" with das-overview
- **AND** the header counts 3 projects

#### Scenario: An organisation without projects
- **GIVEN** the organisation Buttler holds no project
- **WHEN** Brian opens Projects
- **THEN** Buttler does not show

#### Scenario: Every project has an organisation
- **GIVEN** every project has an organisation
- **WHEN** Brian opens Projects
- **THEN** "No organisation" does not show

#### Scenario: The organisation's name opens its page
- **GIVEN** Brian is on Projects
- **WHEN** he clicks the name of Rebl Records' group
- **THEN** Rebl Records' page opens

#### Scenario: A group folds
- **GIVEN** Rebl Records' group is open on Projects
- **WHEN** Brian clicks the rest of its group row
- **THEN** Rebl's row hides and Rebl Records' group row stays

### Requirement: New project asks for a name and an organisation
New project SHALL ask for a name and one organisation, and MUST NOT create a project without both. The new project SHALL show under its organisation on Projects and on the organisation's page.

#### Scenario: A project in Rebl Records
- **GIVEN** Rebl Records holds the project Rebl
- **WHEN** Brian creates the project Fan app with New project and picks Rebl Records
- **THEN** Fan app shows under Rebl Records on Projects
- **AND** Fan app shows on Rebl Records' page

#### Scenario: No organisation picked
- **WHEN** Brian enters the name Fan app in New project and picks no organisation
- **THEN** it cannot create the project

#### Scenario: No name
- **WHEN** Brian picks Rebl Records in New project and enters no name
- **THEN** it cannot create the project

### Requirement: A project's page shows and changes its organisation
A project's page SHALL show its organisation, or "No organisation" when it has none, and SHALL let Brian pick another organisation, after which the project SHALL show under that organisation on Projects and on that organisation's page.

#### Scenario: Moving a project to another organisation
- **GIVEN** the project Fan app belongs to Rebl Records
- **WHEN** Brian picks De App Specialist as its organisation on Fan app's page
- **THEN** Fan app's page shows De App Specialist
- **AND** Fan app shows under De App Specialist on Projects and on De App Specialist's page, and no longer on Rebl Records' page

#### Scenario: Giving a project its first organisation
- **GIVEN** the project das-overview has no organisation
- **WHEN** Brian opens das-overview's page
- **THEN** it shows "No organisation"
- **AND** after he picks De App Specialist, das-overview shows under De App Specialist on Projects

### Requirement: Projects have no status, start date or due date
No page, dialog or filter SHALL show or ask for a project's status, start date or due date.

#### Scenario: Projects
- **WHEN** Brian opens Projects
- **THEN** no project shows a status, a start date or a due date
- **AND** no column, sort or filter offers a project status, start date or due date

#### Scenario: A project's page
- **WHEN** Brian opens a project's page
- **THEN** it shows no status, start date or due date

#### Scenario: New project
- **WHEN** Brian opens New project
- **THEN** it asks for no status, start date or due date

#### Scenario: Issues
- **WHEN** Brian opens the filters on Issues
- **THEN** none filters by project status
