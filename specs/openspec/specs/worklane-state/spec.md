# worklane-state Specification

## Purpose
Lets Brian see whether a worklane is Busy or Free and what it works on now or takes next.

## Requirements

### Requirement: A worklane is Busy while its active issue holds its worktree's branch
A worklane SHALL be Busy while one of its issues that is In Progress or In Review holds its worktree's branch, that issue being its active issue, and Free otherwise; its header SHALL show Busy or Free.

#### Scenario: An agent works REBL-21
- **GIVEN** REBL-21 of Fan accounts and push is In Progress and `~/Worktrees/rebl/rebl/fan-accounts-and-push` is on REBL-21's branch
- **WHEN** Brian opens Fan accounts and push
- **THEN** its header shows Busy

#### Scenario: In Progress without the branch
- **GIVEN** Brian moved REBL-24 of Fan accounts and push to In Progress and no task has worked it, so the worktree is not on REBL-24's branch
- **WHEN** he opens Fan accounts and push
- **THEN** its header shows Free

#### Scenario: The active issue is Done
- **GIVEN** Fan accounts and push is Busy with REBL-21
- **WHEN** REBL-21 becomes Done
- **THEN** its header shows Free

### Requirement: A worklane shows what it works on now or takes next
A worklane's header SHALL show "now: {ID} {title}" for its active issue while it is Busy; while it is Free it SHALL show "next: {ID} {title}" for its first issue that is not Done or Cancelled, taking the releases in their order on the line and then "No release", and within each the issues in the order they were created; it SHALL show neither when every issue of the worklane is Done or Cancelled.

#### Scenario: Now
- **GIVEN** Fan accounts and push is Busy with REBL-21 A fan changes their display name
- **WHEN** Brian opens it
- **THEN** its header shows "now: REBL-21 A fan changes their display name"

#### Scenario: Next in release order
- **GIVEN** Fan accounts and push is Free, v1.0.0 First production release holds its issues REBL-22 (Done) and REBL-23 (Todo), v1.0.1 Membership hardening holds REBL-20 (Todo), and REBL-27 is in no release
- **WHEN** Brian opens it
- **THEN** its header shows "next: REBL-23" and its title

#### Scenario: Next within a release
- **GIVEN** Fan accounts and push is Free and v1.0.0 First production release holds its issues REBL-24 and REBL-21, both Todo, and REBL-21 was created first
- **WHEN** Brian opens it
- **THEN** its header shows "next: REBL-21 A fan changes their display name"

#### Scenario: Nothing left
- **GIVEN** every issue of Agenda and publication is Done or Cancelled
- **WHEN** Brian opens it
- **THEN** its header shows Free and neither "now" nor "next"
