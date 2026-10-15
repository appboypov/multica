## ADDED Requirements

### Requirement: A worklane with an open issue cannot be removed
Remove worklane SHALL be refused while an issue that is neither Done nor Cancelled is in the worklane, naming those issues.

#### Scenario: Open issues
- **GIVEN** REBL-24 (Backlog) and REBL-27 (Todo) are in Fan accounts and push
- **WHEN** Brian removes the worklane Fan accounts and push
- **THEN** it says Fan accounts and push still has the open issues REBL-24 and REBL-27 and removes nothing

### Requirement: A removed worklane's closed issues keep their release under "No worklane"
Removing a worklane SHALL give each of its Done or Cancelled issues "No worklane", keeping its release.

#### Scenario: REBL-22 kept
- **GIVEN** REBL-22 (Done) in v1.0.0 First production release is the only issue of Fan accounts and push, and its worktree holds nothing uncommitted or unmerged
- **WHEN** Brian removes Fan accounts and push
- **THEN** REBL-22's page shows Worklane "No worklane" and Release v1.0.0 First production release
