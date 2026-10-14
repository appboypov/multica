# edit-release Specification

## Purpose
Lets Brian change a release's version, title and description, and set its version by hand or let it follow its place again.

## Requirements

### Requirement: A release's menu offers its actions
Each unshipped release on a worklane's Timeline SHALL have a menu with Edit release, Move earlier, Move later and Remove release.

#### Scenario: The menu of v1.1.0
- **WHEN** Brian opens the menu of v1.1.0 Account and profile on the Timeline of Fan accounts and push
- **THEN** it offers Edit release, Move earlier, Move later and Remove release

### Requirement: Edit release changes a release's version, title and description
Edit release SHALL open the dialog "Edit release · {repository}" filled in with the release's version choice, title and description, with the button "Save release", and SHALL apply the version, title and description rules of Plan release. Saving SHALL change the release on the Timeline of every worklane of the repository.

#### Scenario: A new title and description
- **GIVEN** v1.0.1 Membership hardening has no description
- **WHEN** Brian opens Edit release on it, changes the title to "Memberships", adds the description "Fixes for invitations and memberships found after the first release." and clicks Save release
- **THEN** the Timelines of Fan accounts and push and `main` show v1.0.1 Memberships with that description

#### Scenario: Filled in
- **GIVEN** v1.1.0 Account and profile was planned as Minor after v1.0.1 Membership hardening
- **WHEN** Brian opens Edit release on it
- **THEN** the dialog "Edit release · appboypov/rebl" shows Minor v1.1.0 picked, Patch v1.0.2, Major v2.0.0, the title Account and profile and its description, and "Save release"

#### Scenario: No title
- **WHEN** Brian clears the title in Edit release of v1.1.0 Account and profile
- **THEN** Save release is disabled

### Requirement: A version set by hand is marked and shows what renumbers
A release whose version Brian typed in Custom SHALL show "Set by hand" on the Timeline. While Custom is picked the dialog SHALL say "Set by hand: it keeps this number when the release moves, and must stay higher than the release before it." and, when the later releases' computed versions change, "Later releases renumber: {old} → {new}, …".

#### Scenario: Account and profile set to v1.2.5
- **GIVEN** appboypov/rebl's line holds v1.0.1 Membership hardening, v1.1.0 Account and profile (Minor) and v1.2.0 Notifications (Minor)
- **WHEN** Brian picks Custom in Edit release of Account and profile and types "v1.2.5"
- **THEN** the dialog says "Later releases renumber: v1.2.0 → v1.3.0."
- **AND** after Save release the Timeline shows v1.2.5 Account and profile with "Set by hand", then v1.3.0 Notifications

#### Scenario: Too low
- **GIVEN** v1.0.1 Membership hardening comes before v1.1.0 Account and profile
- **WHEN** Brian picks Custom in Edit release of Account and profile and types "v1.0.1"
- **THEN** the field turns red with "Must be higher than v1.0.1 Membership hardening, the release before it." and Save release is disabled

### Requirement: Picking a bump again gives a computed version
Picking Patch, Minor or Major in Edit release of a release set by hand SHALL give it the version that bump gives after the release before it, drop "Set by hand" on save, and renumber the computed versions after it.

#### Scenario: Back to Minor
- **GIVEN** Account and profile is v1.2.5 set by hand after v1.0.1 Membership hardening, followed by v1.3.0 Notifications (Minor)
- **WHEN** Brian picks Minor in its Edit release and clicks Save release
- **THEN** the Timeline shows v1.1.0 Account and profile without "Set by hand", then v1.2.0 Notifications

### Requirement: An edit that leaves a later release too low is refused
Edit release MUST NOT save a version after which a later release set by hand is not higher than the release before it; Save release SHALL stay disabled with "Then {version} {title} is not higher than {version} {title}, the release before it. Set a different version."

#### Scenario: Notifications set by hand after Account and profile
- **GIVEN** appboypov/rebl's line holds v1.1.0 Account and profile (Minor), then v1.2.5 Notifications set by hand
- **WHEN** Brian picks Custom in Edit release of Account and profile and types "v1.3.0"
- **THEN** it says "Then v1.2.5 Notifications is not higher than v1.3.0 Account and profile, the release before it. Set a different version."
- **AND** Save release is disabled
