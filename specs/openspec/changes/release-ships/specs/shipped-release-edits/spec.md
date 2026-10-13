## ADDED Requirements

### Requirement: Edit release keeps a shipped release's version
Edit release of a shipped release SHALL show its version read only with "Shipped: its tag is fixed." and the line "Shipped as {version}. A new title or description changes the release here and on GitHub; the tag stays.", and SHALL let Brian change its title and description.

#### Scenario: Editing v1.0.0
- **GIVEN** v1.0.0 First production release has shipped
- **WHEN** Brian chooses Edit release on it
- **THEN** Version shows v1.0.0 read only with "Shipped: its tag is fixed."
- **AND** the dialog says "Shipped as v1.0.0. A new title or description changes the release here and on GitHub; the tag stays."

### Requirement: A shipped release's title and description reach its GitHub release
Saving a new title or description of a shipped release SHALL rename its GitHub release to the title and set its note to the description, or to the note from its issues when the description is empty, and MUST NOT change its tag.

#### Scenario: A new description
- **GIVEN** v1.0.1 Membership hardening shipped without a description
- **WHEN** Brian sets its description to "Sign-up and email confirmation" in Edit release and saves
- **THEN** the GitHub release v1.0.1 has the note "Sign-up and email confirmation"
- **AND** the tag v1.0.1 points at the same commit as before

#### Scenario: A new title
- **WHEN** Brian renames shipped v1.0.0 First production release to "Launch" and saves
- **THEN** the Timeline shows v1.0.0 Launch and the GitHub release v1.0.0 is named "Launch"

### Requirement: A shipped release takes no new issues and does not move
A shipped release's menu SHALL show Add issue to release disabled with "Shipped: add it to a later release", Edit release, and Move earlier and Move later disabled with "Shipped: its tag is fixed", and SHALL have no Remove release; it SHALL end without "+ Add issue to release". Move earlier on the first unshipped release SHALL be disabled with "The release before it has shipped".

#### Scenario: The menu of shipped v1.0.0
- **GIVEN** v1.0.0 First production release has shipped
- **WHEN** Brian opens its menu on the Timeline of Fan accounts and push
- **THEN** Add issue to release is disabled with "Shipped: add it to a later release"
- **AND** Move earlier and Move later are disabled with "Shipped: its tag is fixed"
- **AND** Edit release is offered and Remove release is not
- **AND** v1.0.0 ends without "+ Add issue to release"

#### Scenario: Moving the next release earlier
- **GIVEN** v1.0.0 First production release has shipped and v1.0.1 Membership hardening is next
- **WHEN** Brian opens the menu of v1.0.1
- **THEN** Move earlier is disabled with "The release before it has shipped"

### Requirement: An issue can leave a shipped release
An issue in a shipped release SHALL leave it as it leaves any release: through Remove from release on the Timeline, Release and Worklane on its page, and `multica issue update`. The release MUST stay shipped with its version, tag and ship date.

#### Scenario: REBL-22 out of v1.0.0
- **GIVEN** REBL-22 is in v1.0.0 First production release, which has shipped
- **WHEN** Brian picks "No release" in Release on REBL-22's page
- **THEN** REBL-22 shows under "No release" on the Timeline of Fan accounts and push
- **AND** v1.0.0 First production release still shows "Shipped" with its ship date, and the tag v1.0.0 stays
