## ADDED Requirements

### Requirement: Plan release lists this worklane's issues in no release to tick
Plan release SHALL show Issues, "{worklane} · in no release", listing this worklane's issues in no release, each with its type icon, ID, title and status, to tick, none ticked; planning SHALL put the ticked issues in the new release. With none to list, it SHALL say "Every issue of this worklane is in a release. Add issues later from the timeline."

#### Scenario: Two ticked
- **GIVEN** REBL-24 and REBL-27 of Fan accounts and push are in no release
- **WHEN** Brian plans v1.2.0 Notifications from its Timeline with REBL-24 ticked
- **THEN** v1.2.0 lists REBL-24 and "No release" still lists REBL-27

#### Scenario: Nothing to tick
- **GIVEN** every issue of Agenda and publication is in a release
- **WHEN** Brian clicks Plan release on its Timeline
- **THEN** Issues says "Every issue of this worklane is in a release. Add issues later from the timeline."
