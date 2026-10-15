# issues-follow-their-release Specification

## Purpose
Keeps each issue in its release when the release moves, renumbers or is removed.

## Requirements

### Requirement: Issues follow their release when it moves or renumbers
An issue SHALL stay in its release when the release moves on its line or its version renumbers, and SHALL show the release's new version wherever it shows its release.

#### Scenario: Notifications moved earlier
- **GIVEN** REBL-30 is in v1.2.0 Notifications and v1.1.0 Account and profile is before it, neither set by hand
- **WHEN** Brian chooses Move earlier on v1.2.0 Notifications
- **THEN** REBL-30's page shows Release v1.1.0 Notifications and the Timeline lists REBL-30 under v1.1.0 Notifications

### Requirement: The issues of a removed release go back to "No release"
Removing a release SHALL put each of its issues in "No release" on its own worklane, keeping its status.

#### Scenario: v1.2.0 Notifications removed
- **GIVEN** v1.2.0 Notifications holds REBL-30 (Todo) of Fan accounts and push and REBL-31 of `main`
- **WHEN** Brian removes v1.2.0 Notifications
- **THEN** REBL-30 shows in Todo under "No release" on the Timeline of Fan accounts and push and REBL-31 under "No release" on the Timeline of `main`
