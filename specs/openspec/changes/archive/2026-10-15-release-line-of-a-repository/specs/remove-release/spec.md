## ADDED Requirements

### Requirement: Remove release takes an unshipped release off its line
Remove release SHALL take the release off its repository's line and off the Timeline of every worklane of the repository, give every later release with a computed version the version its bump gives at its new place, and keep every version set by hand.

#### Scenario: The first release removed
- **GIVEN** appboypov/rebl has no tag and its line holds v1.0.0 First production release (Major), v1.0.1 Membership hardening (Patch), v1.1.0 Account and profile (Minor) and v1.2.0 Notifications (Minor)
- **WHEN** Brian chooses Remove release on v1.0.0 First production release
- **THEN** the line reads v0.0.1 Membership hardening, v0.1.0 Account and profile, v0.2.0 Notifications
- **AND** v0.0.1 Membership hardening is "Next release" on the Timelines of `main`, Fan accounts and push and Agenda and publication

#### Scenario: A version set by hand stays
- **GIVEN** appboypov/rebl's line holds v1.0.0, v1.0.1 Membership hardening, v1.1.0 Account and profile and v1.2.5 Notifications set by hand
- **WHEN** Brian chooses Remove release on v1.1.0 Account and profile
- **THEN** the line reads v1.0.0, v1.0.1 Membership hardening, v1.2.5 Notifications with "Set by hand"

#### Scenario: The last release removed
- **GIVEN** appboypov/rebl's line holds only v1.0.0 First production release
- **WHEN** Brian chooses Remove release on it
- **THEN** the Timeline of Fan accounts and push says it has no releases yet and shows Plan release
