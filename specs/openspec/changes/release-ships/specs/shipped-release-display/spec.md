## ADDED Requirements

### Requirement: Timeline and List show the last shipped release first
A worklane's Timeline and List SHALL show its repository's last shipped release before the next release, with its version, "Shipped", its ship date, its title, its description and its issues of this worklane, and SHALL fold the older shipped releases behind "Show {N} shipped" before it, which shows them in their order on the line when clicked. Opening the Timeline at an older shipped release SHALL show the folded releases with that release in view.

#### Scenario: One release shipped
- **GIVEN** appboypov/rebl shipped v1.0.0 First production release on 14 Oct 2026 and its line goes on with v1.0.1 Membership hardening and v1.1.0 Account and profile
- **WHEN** Brian opens the Timeline of Fan accounts and push
- **THEN** it shows v1.0.0 with "Shipped" and "14 Oct 2026", then v1.0.1 as Next release, then v1.1.0 as Planned

#### Scenario: Older shipped releases
- **GIVEN** appboypov/rebl shipped v1.0.0 First production release and then v1.0.1 Membership hardening
- **WHEN** Brian opens the List of Fan accounts and push
- **THEN** it shows "Show 1 shipped", then v1.0.1 Membership hardening with "Shipped" and its ship date, then v1.1.0 Account and profile as Next release
- **AND** clicking "Show 1 shipped" shows v1.0.0 First production release before v1.0.1

### Requirement: The worklane's header names the last shipped release
A worklane's header SHALL say "last shipped {version}" with its repository's last shipped release, in place of "nothing shipped yet".

#### Scenario: After v1.0.0 ships
- **GIVEN** the header of Agenda and publication says "nothing shipped yet"
- **WHEN** v1.0.0 First production release of appboypov/rebl ships
- **THEN** the header of Agenda and publication says "last shipped v1.0.0"

### Requirement: Worklane rows show the last shipped release with its ship date
Each worklane row on a repository card, Projects, an organisation's page and Worklanes SHALL show its repository's last shipped release as Shipped with its ship date, in place of "nothing shipped", followed by the next release.

#### Scenario: Fan accounts and push on Projects
- **GIVEN** appboypov/rebl shipped v1.0.0 First production release on 14 Oct 2026 and v1.0.1 Membership hardening is next
- **WHEN** Brian folds Rebl and appboypov/rebl open on Projects
- **THEN** the row Fan accounts and push shows v1.0.0 as Shipped with "14 Oct 2026", then v1.0.1

### Requirement: An issue's release shows that it shipped
The release chip of an issue in a shipped release SHALL show it as shipped on Board cards, List rows and the preview, and the release card on the issue's page SHALL show "Shipped" and the ship date.

#### Scenario: REBL-22 after v1.0.0 ships
- **GIVEN** REBL-22 is in v1.0.0 First production release, shipped on 14 Oct 2026
- **WHEN** Brian opens the Board of Fan accounts and push and then REBL-22's page
- **THEN** REBL-22's card shows the chip v1.0.0 as shipped
- **AND** its page's release card shows v1.0.0, "Shipped", "14 Oct 2026" and First production release

### Requirement: New issue shows the last shipped release disabled
New issue's Release SHALL show the picked worklane's repository's last shipped release before its unshipped releases, disabled as "Shipped".

#### Scenario: New issue from Fan accounts and push
- **GIVEN** appboypov/rebl shipped v1.0.0 First production release and v1.0.1 Membership hardening is next
- **WHEN** Brian clicks New issue on the Timeline of Fan accounts and push and opens Release
- **THEN** v1.0.0 First production release shows disabled as "Shipped", and v1.0.1 Membership hardening is filled in
