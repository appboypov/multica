# add-issue-to-release Specification

## Purpose
Lets Brian add this worklane's issues in no release to an unshipped release from its menu.

## Requirements

### Requirement: Add issue to release adds this worklane's issues in no release
An unshipped release's menu SHALL offer Add issue to release, which SHALL list this worklane's issues in no release, each with its type icon, ID, title and status, to tick, and SHALL put the ticked issues in that release. With none to list, it SHALL say "Every issue of this worklane is in a release."

#### Scenario: REBL-27 into v1.1.0
- **GIVEN** REBL-24 and REBL-27 of Fan accounts and push are in no release
- **WHEN** Brian chooses Add issue to release on v1.1.0 Account and profile and ticks REBL-27
- **THEN** v1.1.0 lists REBL-27 and "No release" lists only REBL-24

#### Scenario: Nothing to add
- **GIVEN** every issue of Agenda and publication is in a release
- **WHEN** Brian chooses Add issue to release on v1.0.1 on its Timeline
- **THEN** it says "Every issue of this worklane is in a release."
