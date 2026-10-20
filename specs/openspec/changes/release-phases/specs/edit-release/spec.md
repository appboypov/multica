## MODIFIED Requirements

### Requirement: A release's menu offers its actions
Each unshipped release that is not on `main` on a worklane's Timeline SHALL have a menu with Edit release, Move earlier, Move later and Remove release. An unshipped release on `main` SHALL have a menu with only Edit release, whose version choice is disabled with "On main: its version is fixed."

#### Scenario: The menu of v1.1.0
- **WHEN** Brian opens the menu of v1.1.0 Account and profile on the Timeline of Fan accounts and push
- **THEN** it offers Edit release, Move earlier, Move later and Remove release

#### Scenario: The menu of a release on main
- **GIVEN** v1.1.0 Account and profile is on main
- **WHEN** Brian opens its menu on the Timeline of Fan accounts and push and then Edit release
- **THEN** the menu offers only Edit release
- **AND** its version choice is disabled with "On main: its version is fixed."
