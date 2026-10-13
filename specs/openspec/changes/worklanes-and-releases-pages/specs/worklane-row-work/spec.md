## ADDED Requirements

### Requirement: A worklane row shows what the worklane works on now or takes next
Each worklane row on a repository card, on Projects, on an organisation's page and on Worklanes SHALL show under the worklane's name its active issue's status, ID and title while the worklane is Busy; while it is Free it SHALL show "Next: {ID} {title}" for the issue the worklane takes next, its first issue that is not Done or Cancelled by release order and then creation order; it SHALL show "Nothing queued" when every issue of the worklane is Done or Cancelled or it has none.

#### Scenario: Busy
- **GIVEN** Fan accounts and push is Busy with REBL-21 A fan changes their display name, which is In Progress
- **WHEN** Brian opens Worklanes
- **THEN** the row Fan accounts and push shows In Progress and "REBL-21 A fan changes their display name"

#### Scenario: Free
- **GIVEN** `main` of appboypov/rebl is Free and takes REBL-27 next
- **WHEN** Brian opens Rebl's page
- **THEN** the row `main` on the card appboypov/rebl shows "Next: REBL-27" and its title

#### Scenario: Nothing queued
- **GIVEN** every issue of Agenda and publication is Done or Cancelled
- **WHEN** Brian folds Rebl and appboypov/rebl open on Projects
- **THEN** the row Agenda and publication shows "Nothing queued"

#### Scenario: The active issue becomes Done
- **GIVEN** Fan accounts and push is Busy with REBL-21 and takes REBL-24 next
- **WHEN** REBL-21 becomes Done and Brian opens Rebl Records' page
- **THEN** the row Fan accounts and push shows "Next: REBL-24" and its title
