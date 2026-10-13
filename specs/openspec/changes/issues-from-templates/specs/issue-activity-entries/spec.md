## ADDED Requirements

### Requirement: An issue's activity records how it was created, typed and placed
An issue's activity SHALL add, with who did it and when:

| When | Entry |
|---|---|
| Created in New issue | "{member} created this {type} from the {template} template" |
| Created in a release | "{actor} planned it in {version} {title} on {worklane}" |
| Its type set on its page | "{member} set the type to {type}" |
| Added to a release from the Timeline, Plan release, its page or the CLI | "{actor} added this to {version} {title}" |
| Taken out of a release from the Timeline, its page or the CLI | "{actor} removed this from {version} {title}" |
| Moved to another worklane | "{actor} moved this to {worklane}" |
| Its release removed | "{version} {title} was removed, this went back to No release" |

#### Scenario: Created from a template in a release
- **WHEN** Brian creates REBL-21 from the Story template in Fan accounts and push and v1.0.0 First production release
- **THEN** its activity shows "Brian created this story from the Story template" and "Brian planned it in v1.0.0 First production release on Fan accounts and push"

#### Scenario: Type set
- **WHEN** Brian sets MUL-12's type to Bug
- **THEN** its activity shows "Brian set the type to Bug"

#### Scenario: Added and removed
- **WHEN** Brian adds REBL-24 to v1.1.0 Account and profile and later chooses Remove from release on it
- **THEN** REBL-24's activity shows "Brian added this to v1.1.0 Account and profile" and then "Brian removed this from v1.1.0 Account and profile"

#### Scenario: Moved to another worklane
- **WHEN** Brian picks Agenda and publication as REBL-21's worklane
- **THEN** REBL-21's activity shows "Brian moved this to Agenda and publication"

#### Scenario: Its release removed
- **GIVEN** REBL-30 is in v1.2.0 Notifications
- **WHEN** Brian removes v1.2.0 Notifications
- **THEN** REBL-30's activity shows "v1.2.0 Notifications was removed, this went back to No release" with Brian as who did it
