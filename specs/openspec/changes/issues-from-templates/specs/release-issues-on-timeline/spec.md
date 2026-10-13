## ADDED Requirements

### Requirement: Each release on the Timeline shows its issues of this worklane with their progress
Each release on a worklane's Timeline SHALL list the release's issues of this worklane, each with its status, type icon, ID and title, and "{done} of {total} done" over them; a release that holds none of this worklane's issues SHALL show "No issues of this worklane". Issues of the release in other worklanes MUST NOT show on this Timeline.

#### Scenario: v1.0.0 on Fan accounts and push
- **GIVEN** v1.0.0 First production release holds REBL-21 (Todo) and REBL-22 (Done) of Fan accounts and push and REBL-26 of Agenda and publication
- **WHEN** Brian opens the Timeline of Fan accounts and push
- **THEN** v1.0.0 lists REBL-21 and REBL-22 with "1 of 2 done" and not REBL-26

#### Scenario: A release without this worklane's issues
- **GIVEN** v1.0.1 Membership hardening holds only issues of `main`
- **WHEN** Brian opens the Timeline of Fan accounts and push
- **THEN** v1.0.1 shows "No issues of this worklane"

### Requirement: The Timeline shows "No release" with this worklane's issues in no release
After the releases and Plan release, a worklane's Timeline SHALL show "No release" with the number of this worklane's issues in no release and those issues; it SHALL show it also for a repository that does not release.

#### Scenario: Two issues in no release
- **GIVEN** REBL-24 and REBL-27 of Fan accounts and push are in no release
- **WHEN** Brian opens its Timeline
- **THEN** after Plan release it shows "No release" with "2 issues", REBL-24 and REBL-27

#### Scenario: A repository that does not release
- **GIVEN** appboypov/rebl-site does not release and its worklane `main` holds REBL-40
- **WHEN** Brian opens the Timeline of `main` of appboypov/rebl-site
- **THEN** it shows "No release" with REBL-40

### Requirement: The + of a release opens New issue in that release
Each unshipped release on the Timeline SHALL end with "+ Add issue to release", which SHALL open New issue with this worklane and that release filled in.

#### Scenario: New issue in v1.1.0
- **WHEN** Brian clicks + Add issue to release under v1.1.0 Account and profile on the Timeline of Fan accounts and push
- **THEN** New issue opens with Worklane Fan accounts and push and Release v1.1.0 Account and profile

### Requirement: Remove from release puts an issue in "No release"
An issue in an unshipped release on the Timeline SHALL have Remove from release in its menu, which SHALL take it out of the release and show it under "No release", keeping its worklane and status.

#### Scenario: REBL-24 out of v1.1.0
- **GIVEN** REBL-24 (Todo) of Fan accounts and push is in v1.1.0 Account and profile
- **WHEN** Brian chooses Remove from release in REBL-24's menu on the Timeline
- **THEN** REBL-24 shows under "No release" in Todo and v1.1.0 no longer lists it

#### Scenario: An issue in no release
- **WHEN** Brian opens the menu of REBL-27, which is in no release
- **THEN** it has no Remove from release
