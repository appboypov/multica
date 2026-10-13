## ADDED Requirements

### Requirement: A repository with unshipped releases cannot stop releasing
Edit repository MUST NOT save "This repository does not release" while the repository's line holds unshipped releases; it SHALL say "Remove its unshipped releases first: {version} {title}, …", naming each, and keep Save disabled.

#### Scenario: appboypov/rebl has planned releases
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release and v1.0.1 Membership hardening, neither shipped
- **WHEN** Brian picks "This repository does not release" in Edit repository of appboypov/rebl
- **THEN** it says "Remove its unshipped releases first: v1.0.0 First production release, v1.0.1 Membership hardening."
- **AND** Save is disabled

#### Scenario: Releases removed first
- **GIVEN** Brian removed every release of appboypov/rebl
- **WHEN** he picks "This repository does not release" in Edit repository and saves
- **THEN** the Timeline of Fan accounts and push shows "appboypov/rebl does not release, so its worklanes plan no releases."

#### Scenario: Releasing again
- **GIVEN** appboypov/rebl-site does not release and has the last tag v2.1.0
- **WHEN** Brian picks "Tags only" in its Edit repository, saves and clicks Plan release on the Timeline of its worklane `main`
- **THEN** the dialog offers Patch v2.1.1, Minor v2.2.0 and Major v3.0.0
