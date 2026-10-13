## ADDED Requirements

### Requirement: The issue page shows the type, the ID and the description as Preview or Markdown
An issue's page SHALL show its type icon and name with its ID above its title, its description as Preview, the rendered Markdown, or as Markdown, its text, and under it "Code and technical design go in the change, not in the issue."

#### Scenario: REBL-21
- **GIVEN** Brian created REBL-21 "A fan changes their display name" from the Story template
- **WHEN** he opens its page
- **THEN** it shows Story REBL-21 above the title and the description as Preview with "👤 User Story" as a heading
- **AND** Markdown shows "## 👤 User Story" as text
- **AND** under it says "Code and technical design go in the change, not in the issue."

### Requirement: The issue page shows Type, Worklane and Release next to Multica's other properties
An issue's page SHALL show Type, Worklane and Release next to Multica's other properties of the issue. Worklane SHALL show the worklane with its repository, or "No worklane", and Release the release's version and title, or "No release"; each SHALL link to the worklane's Timeline.

#### Scenario: In a worklane and a release
- **GIVEN** REBL-21 is in Fan accounts and push and v1.0.0 First production release
- **WHEN** Brian opens its page
- **THEN** it shows Type Story, Worklane Fan accounts and push of appboypov/rebl and Release v1.0.0 First production release next to Status, Priority and Assignee
- **AND** clicking the worklane or the release opens the Timeline of Fan accounts and push

#### Scenario: Without a worklane
- **WHEN** Brian opens the page of the spike "Can System One share booking data with us?"
- **THEN** it shows Worklane "No worklane" and Release "No release"

### Requirement: Worklane and Release on the issue page follow New issue's rules
Worklane on the issue page SHALL offer every worklane and "No worklane", and Release the unshipped releases of the worklane's repository and "No release", or "Pick a worklane first" without a worklane. Picking a worklane of another repository, or "No worklane", SHALL set the release to "No release"; picking a worklane of the same repository SHALL keep the release. Picking a worklane SHALL move the issue to the worklane's project.

#### Scenario: An issue from before into a worklane
- **GIVEN** MUL-12 has No worklane and No release
- **WHEN** Brian picks Fan accounts and push as its worklane and v1.1.0 Account and profile as its release
- **THEN** MUL-12 shows under v1.1.0 on the Timeline of Fan accounts and push and belongs to Rebl

#### Scenario: Another worklane of the same repository
- **GIVEN** REBL-21 is in Fan accounts and push and v1.0.0
- **WHEN** Brian picks Agenda and publication as its worklane
- **THEN** REBL-21 keeps v1.0.0 and shows under v1.0.0 on the Timeline of Agenda and publication

#### Scenario: A worklane of another repository
- **GIVEN** REBL-21 is in Fan accounts and push and v1.0.0
- **WHEN** Brian picks `main` of appboypov/rebl-admin
- **THEN** REBL-21 shows Release "No release"

### Requirement: The issue page shows its release's card
An issue in a release SHALL show on its page a card with the release's version and state, Next release or Planned, its title, its description, the worklane and the repository, its progress "{done} of {total} done" over the worklane's issues in the release, and Open timeline, which opens the worklane's Timeline.

#### Scenario: REBL-21's release card
- **GIVEN** v1.0.0 First production release is appboypov/rebl's next release, with the description "The fan app in the stores" and the issues REBL-21 and REBL-22 of Fan accounts and push, REBL-22 Done
- **WHEN** Brian opens REBL-21's page
- **THEN** the card shows v1.0.0, Next release, First production release, "The fan app in the stores", Fan accounts and push, appboypov/rebl and "1 of 2 done"
- **AND** Open timeline opens the Timeline of Fan accounts and push

#### Scenario: No release, no card
- **WHEN** Brian opens the page of the chore "Renew the Apple push certificate", which is in No release
- **THEN** it shows no release card
