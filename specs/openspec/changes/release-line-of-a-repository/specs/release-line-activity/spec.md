## ADDED Requirements

### Requirement: Every worklane's activity records each change to its repository's line
Planning, editing, moving and removing a release SHALL add one entry to the activity of every worklane of its repository, with who did it and when: "{member} planned {version} {title}", "{member} edited {version} {title}", "{member} moved {version} {title} earlier" or "later", and "{member} removed {version} {title}", naming the release by its version after the change. When versions changed, the entry SHALL list each changed release as "{title} {old version} → {new version}".

#### Scenario: Planned
- **GIVEN** appboypov/rebl has the worklanes `main`, Fan accounts and push and Agenda and publication
- **WHEN** Brian plans v1.0.1 Membership hardening from the Timeline of Fan accounts and push at 14:10 on 13 October 2026
- **THEN** the activity of each of the three worklanes shows "Brian planned v1.0.1 Membership hardening" at 14:10 on 13 October 2026

#### Scenario: Moved with renumbering
- **GIVEN** appboypov/rebl's line holds v1.1.0 Account and profile (Minor) and then v1.2.0 Notifications (Minor)
- **WHEN** Brian chooses Move earlier on v1.2.0 Notifications
- **THEN** the activity of `main` shows "Brian moved v1.1.0 Notifications earlier" with "Notifications v1.2.0 → v1.1.0" and "Account and profile v1.1.0 → v1.2.0"

#### Scenario: Edited to a version set by hand
- **GIVEN** appboypov/rebl's line holds v1.1.0 Account and profile (Minor) and then v1.2.0 Notifications (Minor)
- **WHEN** Brian sets Account and profile by hand to v1.2.5 and saves
- **THEN** the activity of Agenda and publication shows "Brian edited v1.2.5 Account and profile" with "Account and profile v1.1.0 → v1.2.5" and "Notifications v1.2.0 → v1.3.0"

#### Scenario: Edited without a version change
- **WHEN** Brian changes the description of v1.0.1 Membership hardening and saves
- **THEN** the activity of Fan accounts and push shows "Brian edited v1.0.1 Membership hardening" without a version change

#### Scenario: Removed
- **WHEN** Brian removes v1.2.0 Notifications, the last release on appboypov/rebl's line
- **THEN** the activity of every worklane of appboypov/rebl shows "Brian removed v1.2.0 Notifications"

#### Scenario: A worklane created later
- **GIVEN** Brian planned v1.0.0 First production release on appboypov/rebl
- **WHEN** he then creates the worklane Fan app on appboypov/rebl and opens its page
- **THEN** its activity shows "Brian created Fan app on Brian's Mac" and no entry for v1.0.0
- **AND** its Timeline shows v1.0.0 First production release
