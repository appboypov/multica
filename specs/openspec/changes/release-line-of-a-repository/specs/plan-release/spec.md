## ADDED Requirements

### Requirement: Plan release offers a version after the last release on the line
Plan release SHALL open the dialog "Plan release · {repository}" with Version offering Patch, Minor and Major, each showing the version its bump gives after the last release on the line, or after the repository's last tag when the line is empty, or after v0.0.0 when there is no tag either, and Custom, which opens a field to type the version by hand. Patch SHALL be picked when the dialog opens.

#### Scenario: After the last planned release
- **GIVEN** appboypov/rebl's line ends with v1.1.0 Account and profile
- **WHEN** Brian clicks Plan release on the Timeline of Fan accounts and push
- **THEN** the dialog "Plan release · appboypov/rebl" offers Patch v1.1.1, Minor v1.2.0, Major v2.0.0 and Custom, with Patch picked
- **AND** it says "Follows v1.1.0 Account and profile. Moving it renumbers it."

#### Scenario: The first release after the last tag
- **GIVEN** appboypov/rebl-admin has the last tag v0.4.2 and no releases
- **WHEN** Brian clicks Plan release on the Timeline of its worklane `main`
- **THEN** the dialog offers Patch v0.4.3, Minor v0.5.0 and Major v1.0.0
- **AND** it says "Follows v0.4.2, the last tag. Moving it renumbers it."

#### Scenario: The first release without a tag
- **GIVEN** appboypov/rebl has no tag and no releases
- **WHEN** Brian clicks Plan release on the Timeline of Fan accounts and push
- **THEN** the dialog offers Patch v0.0.1, Minor v0.1.0 and Major v1.0.0
- **AND** it says "The first release of this repository."

### Requirement: Plan release asks for a title and a description
Plan release SHALL ask for a required Title and an optional Description, and SHALL ask for no goal and no date. Its button SHALL name the version it plans, "Plan {version}", and stay disabled until the release has a title and a valid version.

#### Scenario: No title
- **GIVEN** the Plan release dialog of appboypov/rebl with Patch v1.1.1 picked and Title empty
- **THEN** the button "Plan v1.1.1" is disabled

#### Scenario: Title without description
- **WHEN** Brian types the title "Notifications", leaves Description empty and picks Minor
- **THEN** the button "Plan v1.2.0" is enabled
- **AND** the dialog shows no goal and no date field

### Requirement: A version typed by hand must be a version higher than the release before it
Custom SHALL accept a version written as `v1.4.0` or `1.4.0` and save it as `v1.4.0`. When the typed text is not such a version the field SHALL say "Use the form v1.4.0."; when the version is not higher than the last release on the line, or the last tag on an empty line, the field SHALL turn red with "Must be higher than {version} {title}, the release before it." or "Must be higher than {tag}, the last tag." In both cases the Plan button MUST stay disabled.

#### Scenario: Not a version
- **WHEN** Brian picks Custom in the Plan release dialog of appboypov/rebl and types "1.4"
- **THEN** the field says "Use the form v1.4.0." and the Plan button is disabled

#### Scenario: Written without v
- **GIVEN** appboypov/rebl's line ends with v1.1.0 Account and profile
- **WHEN** Brian picks Custom, types "1.4.0" and the title "Notifications"
- **THEN** the button reads "Plan v1.4.0" and is enabled

#### Scenario: Too low after the last release
- **GIVEN** appboypov/rebl's line ends with v1.1.0 Account and profile
- **WHEN** Brian picks Custom and types "v1.0.5"
- **THEN** the field turns red with "Must be higher than v1.1.0 Account and profile, the release before it." and the Plan button is disabled

#### Scenario: Too low after the last tag
- **GIVEN** appboypov/rebl-admin has the last tag v0.4.2 and no releases
- **WHEN** Brian picks Custom and types "v0.4.2"
- **THEN** the field turns red with "Must be higher than v0.4.2, the last tag." and the Plan button is disabled

### Requirement: Plan release says what happens when it ships
Plan release SHALL show the read-only line "When its issues are done and merged, Multica tags {version} on {default branch}, writes the release note and publishes the GitHub release. It gets its date when it ships." with the version it plans, and MUST NOT mention a pass.

#### Scenario: Patch picked
- **GIVEN** the Plan release dialog of appboypov/rebl, whose default branch is `main`, with Patch v1.1.1 picked
- **THEN** it shows "When its issues are done and merged, Multica tags v1.1.1 on main, writes the release note and publishes the GitHub release. It gets its date when it ships."

### Requirement: A planned release joins the end of its repository's line
Planning a release SHALL add it after the last release on its repository's line, on the Timeline of every worklane of that repository, and close the dialog.

#### Scenario: Notifications planned
- **GIVEN** appboypov/rebl's line ends with v1.1.0 Account and profile
- **WHEN** Brian plans Minor v1.2.0 with the title "Notifications" from the Timeline of Fan accounts and push
- **THEN** that Timeline shows v1.2.0 Notifications, Planned, after v1.1.0
- **AND** the Timelines of `main` and Agenda and publication show v1.2.0 Notifications after v1.1.0

#### Scenario: The first release becomes next
- **GIVEN** appboypov/rebl has no tag and no releases
- **WHEN** Brian plans Major v1.0.0 with the title "First production release"
- **THEN** the Timeline shows v1.0.0 First production release as "Next release"
