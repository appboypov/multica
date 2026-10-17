# release-owner Specification

## Purpose
Lets Brian give an unshipped release one optional agent or squad owner that follows the rules of an issue assignee.

## Requirements

### Requirement: A release has an optional owner
A release SHALL have at most one owner, an agent or a squad of its workspace, and SHALL have none until one is set. A release without an owner MUST plan, edit, move, remove, take issues and ship exactly as a release does without this change.

#### Scenario: Planned without an owner
- **WHEN** Brian plans v1.1.0 Account and profile on appboypov/rebl without picking an owner
- **THEN** `multica release get` for v1.1.0 shows no owner
- **AND** no run starts for any agent

#### Scenario: A release without an owner ships as before
- **GIVEN** v1.0.1 Membership hardening has no owner and every issue in it is done with its pull requests merged
- **WHEN** Brian runs `multica release ship` for v1.0.1
- **THEN** v1.0.1 is shipped

### Requirement: Plan release and Edit release set the owner
Plan release and Edit release SHALL offer an Owner field listing the workspace's agents and squads, with "No owner" as its default in Plan release and the current owner filled in Edit release. Saving SHALL set, change or remove the owner.

#### Scenario: Owner picked in Plan release
- **WHEN** Brian plans v1.1.0 Account and profile and picks Pieter as Owner
- **THEN** v1.1.0 on the Timeline of every worklane of appboypov/rebl shows Pieter as its owner

#### Scenario: Owner changed in Edit release
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** Brian opens Edit release on v1.1.0, picks the squad Build squad and clicks Save release
- **THEN** v1.1.0 shows Build squad as its owner

#### Scenario: Owner removed in Edit release
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** Brian picks No owner in Edit release on v1.1.0 and clicks Save release
- **THEN** v1.1.0 shows no owner

#### Scenario: Members are not offered
- **WHEN** Brian opens the Owner field in Plan release
- **THEN** it lists agents and squads and no workspace members

### Requirement: The owner shows wherever the release does
A release with an owner SHALL show the owner's name and avatar on its release in every worklane's Timeline and on its row in Releases, and `multica release get` and `multica release list` SHALL include the owner's type, id and name.

#### Scenario: Releases row
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** Brian opens Releases
- **THEN** the row of v1.1.0 shows Pieter as its owner

#### Scenario: CLI
- **GIVEN** Build squad owns v1.1.0 Account and profile
- **WHEN** an agent runs `multica release get` for v1.1.0 with `--output json`
- **THEN** the release shows owner type `squad`, Build squad's id and the name Build squad

### Requirement: The CLI assigns and unassigns a release
`multica release assign <release> --to <agent or squad>` SHALL make that agent or squad the release's owner, and `multica release assign <release> --unassign` SHALL remove the owner, with the same effects as Edit release.

#### Scenario: Assign by CLI
- **WHEN** Brian runs `multica release assign` for v1.1.0 Account and profile with `--to Pieter`
- **THEN** Pieter owns v1.1.0

#### Scenario: Unassign by CLI
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** Brian runs `multica release assign` for v1.1.0 with `--unassign`
- **THEN** v1.1.0 has no owner

### Requirement: An owner follows the rules of an issue assignee
Setting an owner MUST be refused for an agent or squad that the same person could not assign an issue to, with the same message issue assignment gives: an archived agent with "cannot assign to archived agent", and an agent or squad they lack permission for with "you do not have permission to assign work to this agent" or "... to this squad".

#### Scenario: Private agent
- **GIVEN** the agent Joeri is private to Anouk
- **WHEN** Brian makes Joeri owner of v1.1.0 Account and profile
- **THEN** the change is refused with "you do not have permission to assign work to this agent"
- **AND** v1.1.0 keeps its owner

#### Scenario: Archived agent
- **GIVEN** the agent Kees is archived
- **WHEN** Brian runs `multica release assign` for v1.1.0 with `--to Kees`
- **THEN** it is refused with "cannot assign to archived agent"

### Requirement: Only an unshipped release takes an owner change
Setting, changing or removing the owner of a shipped release MUST be refused with "{version} {title} is shipped".

#### Scenario: Shipped release
- **GIVEN** v1.0.0 First production release is shipped
- **WHEN** Brian runs `multica release assign` for v1.0.0 with `--to Pieter`
- **THEN** it is refused with "v1.0.0 First production release is shipped"

### Requirement: Every worklane's activity records each owner change
Setting, changing and removing a release's owner SHALL add one entry to the activity of every worklane of its repository, with who did it and when: "{actor} made {owner} owner of {version} {title}", "{actor} made {owner} owner of {version} {title} instead of {previous owner}" and "{actor} removed {owner} as owner of {version} {title}".

#### Scenario: Owner set
- **GIVEN** appboypov/rebl has the worklanes `main` and Fan accounts and push
- **WHEN** Brian makes Pieter owner of v1.1.0 Account and profile at 10:05 on 19 October 2026
- **THEN** the activity of both worklanes shows "Brian made Pieter owner of v1.1.0 Account and profile" at 10:05 on 19 October 2026

#### Scenario: Owner changed
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** Brian makes Build squad its owner
- **THEN** the activity of `main` shows "Brian made Build squad owner of v1.1.0 Account and profile instead of Pieter"

#### Scenario: Owner removed
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** Brian removes the owner
- **THEN** the activity of `main` shows "Brian removed Pieter as owner of v1.1.0 Account and profile"
