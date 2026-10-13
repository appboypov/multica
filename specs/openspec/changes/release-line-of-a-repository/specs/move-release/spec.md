## ADDED Requirements

### Requirement: Move earlier and Move later swap a release with its neighbour
Move earlier and Move later SHALL swap the release with the release before or after it on its repository's line, give every release with a computed version the version its bump gives at its new place, keep every version set by hand, and show the new order on the Timeline of every worklane of the repository.

#### Scenario: Notifications moved earlier
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release, v1.0.1 Membership hardening, v1.1.0 Account and profile (Minor) and v1.2.0 Notifications (Minor), none set by hand
- **WHEN** Brian chooses Move earlier on v1.2.0 Notifications on the Timeline of Fan accounts and push
- **THEN** the line reads v1.0.0 First production release, v1.0.1 Membership hardening, v1.1.0 Notifications, v1.2.0 Account and profile
- **AND** the Timeline of Agenda and publication shows that order and those versions

#### Scenario: A version set by hand keeps its number
- **GIVEN** appboypov/rebl's line holds v1.0.0, v1.0.1 Membership hardening, v1.1.0 Account and profile (Minor) and v1.2.5 Notifications set by hand
- **WHEN** Brian chooses Move earlier on v1.2.5 Notifications
- **THEN** the line reads v1.0.0, v1.0.1 Membership hardening, v1.2.5 Notifications with "Set by hand", v1.3.0 Account and profile

### Requirement: A move that breaks the line is refused with its reason
Move earlier SHALL be disabled with "It is the first release" on the first release, and Move later with "It is the last release" on the last. A move after which a release set by hand is not higher than the release before it MUST be disabled with "Then {version} {title} is not higher than {version} {title}, the release before it".

#### Scenario: The first release
- **WHEN** Brian opens the menu of v1.0.0 First production release, the first on appboypov/rebl's line
- **THEN** Move earlier is disabled with "It is the first release"

#### Scenario: The last release
- **WHEN** Brian opens the menu of v1.2.0 Notifications, the last on appboypov/rebl's line
- **THEN** Move later is disabled with "It is the last release"

#### Scenario: A version set by hand would end up too low
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release, v1.0.1 Membership hardening set by hand, v1.1.0 Account and profile (Minor) and v1.2.0 Notifications
- **WHEN** Brian opens the menu of v1.0.1 Membership hardening
- **THEN** Move later is disabled with "Then v1.0.1 Membership hardening is not higher than v1.1.0 Account and profile, the release before it"
- **AND** the line stays as it was
