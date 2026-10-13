## ADDED Requirements

### Requirement: Worklane rows show the next release's progress and the planned releases that hold the worklane's issues
Each worklane row on a repository card, on Projects and on an organisation's page SHALL show after the next release "{done}/{total}" over the worklane's issues in it, or no progress when it holds none of them, and then up to two planned releases after it that hold the worklane's issues, in their order.

#### Scenario: Fan accounts and push on Projects
- **GIVEN** v1.0.0 First production release, the next, holds REBL-21 and REBL-22 of Fan accounts and push with REBL-22 Done, v1.0.1 holds none of its issues, and v1.1.0 and v1.2.0 each hold one
- **WHEN** Brian folds Rebl and appboypov/rebl open on Projects
- **THEN** the row Fan accounts and push shows "nothing shipped", v1.0.0 with "1/2", v1.1.0 and v1.2.0

#### Scenario: The next release holds none of the worklane's issues
- **GIVEN** v1.0.0 First production release holds no issue of Agenda and publication
- **WHEN** Brian opens Rebl's page
- **THEN** the row Agenda and publication shows v1.0.0 without progress
