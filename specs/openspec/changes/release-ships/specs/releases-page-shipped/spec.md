## ADDED Requirements

### Requirement: Releases shows the last shipped release before the unshipped ones
Releases SHALL show under each repository its last shipped release first, with its version as Shipped, its title, its ship date, its progress and its worklanes, and SHALL fold the older shipped releases behind "Show {N} shipped" before it, which shows them in their order on the line when clicked. The header, organisation and project counts SHALL count every release on the line, shipped ones included.

#### Scenario: One release shipped
- **GIVEN** appboypov/rebl shipped v1.0.0 First production release on 14 Oct 2026, and its line goes on with v1.0.1 Membership hardening and v1.1.0 Account and profile
- **WHEN** Brian opens Releases
- **THEN** the rows under appboypov/rebl are v1.0.0 as Shipped with "14 Oct 2026", v1.0.1 as Next release and v1.1.0 as Planned
- **AND** the header counts 3 releases

#### Scenario: Older shipped releases
- **GIVEN** appboypov/rebl shipped v1.0.0 First production release and then v1.0.1 Membership hardening, and v1.1.0 Account and profile is next
- **WHEN** Brian opens Releases
- **THEN** appboypov/rebl shows "Show 1 shipped", then v1.0.1 as Shipped with its ship date, then v1.1.0 as Next release
- **AND** clicking "Show 1 shipped" shows v1.0.0 First production release before v1.0.1

#### Scenario: A shipped release's row
- **GIVEN** v1.0.0 First production release shipped with REBL-76 of `main` and REBL-22 of Fan accounts and push
- **WHEN** Brian clicks its row on Releases
- **THEN** the Timeline of `main` opens with v1.0.0 First production release in view
