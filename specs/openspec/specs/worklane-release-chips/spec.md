# worklane-release-chips Specification

## Purpose
Lets Brian see each worklane's last and next release on a repository card, Projects and an organisation's page.

## Requirements

### Requirement: Worklane rows show the last and next release
Each worklane row on a repository card, on Projects and on an organisation's page SHALL show its repository's last shipped release, or "nothing shipped" while none has shipped, and then the repository's next release, the first unshipped one on its line; it SHALL show "No releases planned" while the line is empty and "Does not release" for a repository that does not release. Clicking a release SHALL open the worklane's Timeline. Projects and an organisation's page SHALL head the column "Last · Next · Planned releases".

#### Scenario: appboypov/rebl with releases
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release and v1.0.1 Membership hardening, neither shipped
- **WHEN** Brian opens Rebl's page
- **THEN** the rows `main`, Fan accounts and push and Agenda and publication each show "nothing shipped" and then v1.0.0 as the next release

#### Scenario: On Projects
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release
- **WHEN** Brian folds Rebl and appboypov/rebl open on Projects
- **THEN** the column is headed "Last · Next · Planned releases" and each worklane row shows "nothing shipped" and v1.0.0

#### Scenario: Opening the Timeline from a release
- **WHEN** Brian clicks v1.0.0 on the row Fan accounts and push on Rebl Records' page
- **THEN** the Timeline of Fan accounts and push opens

#### Scenario: No releases
- **GIVEN** appboypov/rebl-admin releases and has no releases on its line
- **THEN** its worklane `main` shows "No releases planned" on its project's page, Projects and its organisation's page

#### Scenario: Does not release
- **GIVEN** appboypov/rebl-site is set to "This repository does not release"
- **THEN** its worklane `main` shows "Does not release" on its project's page, Projects and its organisation's page
