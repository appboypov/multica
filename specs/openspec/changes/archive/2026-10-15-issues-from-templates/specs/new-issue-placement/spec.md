## ADDED Requirements

### Requirement: New issue shows Type, Worklane, Release and Priority next to Multica's other properties
New issue SHALL show under the title the chips Type, Worklane, Release and Priority, and SHALL keep Multica's other properties of a new issue, such as the assignee, the status, labels, dates, the parent and attachments.

#### Scenario: From the sidebar
- **WHEN** Brian opens New issue from the sidebar
- **THEN** it shows the chips Story, No worklane, No release and No priority
- **AND** it offers the assignee, the status, labels, dates, the parent and attachments as Multica's New issue does

### Requirement: Where New issue opens fills in the worklane and its next release
New issue opened from a worklane SHALL fill in that worklane and its repository's next release, the first unshipped release on its line; opened from a project's page it SHALL fill in `main` of the project's first repository and that repository's next release; opened from elsewhere it SHALL fill in no worklane. Picking a worklane SHALL fill in its repository's next release, or "No release" when the line has no unshipped release.

#### Scenario: From a worklane
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release, unshipped, and v1.0.1 Membership hardening
- **WHEN** Brian clicks New issue on the Timeline of Fan accounts and push
- **THEN** the Worklane chip says Fan accounts and push and the Release chip v1.0.0 First production release

#### Scenario: From a project
- **GIVEN** Rebl's first repository is appboypov/rebl
- **WHEN** Brian clicks New issue on Rebl's page
- **THEN** the Worklane chip says main of appboypov/rebl and the Release chip v1.0.0 First production release

#### Scenario: Picking a worklane without releases
- **GIVEN** appboypov/rebl-admin has no releases on its line
- **WHEN** Brian picks its worklane `main` in New issue
- **THEN** the Release chip says No release

### Requirement: The release is one of the worklane's repository's unshipped releases or none
The Release chip SHALL offer the unshipped releases of the picked worklane's repository in their order, each with its version and title, and "No release"; it SHALL say "This worklane has no releases planned" when the line has none, and "Pick a worklane first" without a worklane.

#### Scenario: Releases offered
- **GIVEN** appboypov/rebl's line holds v1.0.0, v1.0.1, v1.1.0 and v1.2.0, none shipped
- **WHEN** Brian opens the Release chip with Fan accounts and push picked
- **THEN** it offers v1.0.0 First production release, v1.0.1 Membership hardening, v1.1.0 Account and profile, v1.2.0 Notifications and No release

#### Scenario: No worklane
- **WHEN** Brian opens the Release chip with No worklane
- **THEN** it says "Pick a worklane first" and offers only No release

#### Scenario: Another worklane of another repository
- **GIVEN** Brian picked Fan accounts and push and v1.1.0 Account and profile
- **WHEN** he picks `main` of appboypov/rebl-admin, which has no releases
- **THEN** the Release chip says No release and says "This worklane has no releases planned" when opened

### Requirement: A new issue starts in Todo with a release and in Backlog without
New issue SHALL create the issue in the status Brian picked, and otherwise in Todo when it has a release and in Backlog when it has none.

#### Scenario: With a release
- **WHEN** Brian creates the story "A fan changes their display name" in Fan accounts and push and v1.0.0 without picking a status
- **THEN** it is in Todo

#### Scenario: Without a release
- **WHEN** Brian creates the chore "Renew the Apple push certificate" in Fan accounts and push with No release without picking a status
- **THEN** it is in Backlog

#### Scenario: A status picked
- **WHEN** Brian creates a story in v1.0.0 and picks In progress
- **THEN** it is in In progress

### Requirement: An issue belongs to its worklane's project
An issue created in a worklane SHALL belong to the project of the worklane's repository.

#### Scenario: Fan accounts and push
- **WHEN** Brian creates REBL-21 in Fan accounts and push of appboypov/rebl
- **THEN** REBL-21 belongs to the project Rebl

### Requirement: An issue without a worklane is kept under "No worklane"
New issue SHALL create an issue with No worklane and No release, and Multica SHALL keep it, show "No worklane" as its worklane on its page, and list it with "No worklane" in ⌘K search.

#### Scenario: Created without a worklane
- **WHEN** Brian creates the spike "Can System One share booking data with us?" from the sidebar without a worklane
- **THEN** it is created in Backlog with No worklane and No release
- **AND** ⌘K search for "System One" lists it with "No worklane"
