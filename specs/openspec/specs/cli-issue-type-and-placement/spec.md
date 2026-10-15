# cli-issue-type-and-placement Specification

## Purpose
Lets an agent or the CLI create an issue with or without a type, in a worklane and a release, and move an issue to another worklane or release.

## Requirements

### Requirement: The CLI creates an issue with one of the eight types or none
`multica issue create` SHALL take an optional `--type`, one of epic, journey, story, bug, enhancement, refactor, chore and spike, and SHALL refuse any other value, naming the eight. Without `--type` the issue SHALL have no type.

#### Scenario: A bug
- **WHEN** an agent runs `multica issue create` with the title "Invitation link opens an empty screen" and `--type bug`
- **THEN** the issue is a bug and shows the Bug icon

#### Scenario: No type
- **WHEN** an agent runs `multica issue create` with a title and no `--type`
- **THEN** the issue shows the No type icon

#### Scenario: An unknown type
- **WHEN** an agent runs `multica issue create` with `--type task`
- **THEN** it fails naming epic, journey, story, bug, enhancement, refactor, chore and spike, and creates no issue

### Requirement: A description the CLI sends is kept as written
Multica SHALL keep a description the CLI sends as written, with or without a type, and MUST NOT refuse it for missing template headings.

#### Scenario: A story without template headings
- **WHEN** an agent runs `multica issue create` with `--type story` and the description "Fans change their name on their profile."
- **THEN** the story's description is "Fans change their name on their profile."

### Requirement: A typed issue without a description gets its template's Markdown
An issue created with a type and no description SHALL get its template's Markdown, each section with its heading and question and no answers.

#### Scenario: A bug without a description
- **WHEN** an agent runs `multica issue create` with `--type bug` and no description
- **THEN** the bug's description is the Bug template's Markdown, from "## 🔗 Dependencies" to "## 📚 References"

### Requirement: The CLI puts an issue in a worklane and a release
`multica issue create` and `multica issue update` SHALL take `--worklane` and `--release` and apply the issue page's rules: the release MUST be an unshipped release of the worklane's repository, `--release` without a worklane SHALL fail, the issue SHALL belong to the worklane's project, and `multica issue update` SHALL be able to set "No release" and "No worklane". A refused value SHALL create or change nothing.

#### Scenario: Created in Fan accounts and push and v1.0.1
- **WHEN** an agent runs `multica issue create` with `--type bug`, the worklane Fan accounts and push of appboypov/rebl and the release v1.0.1
- **THEN** the bug shows under v1.0.1 Membership hardening on the Timeline of Fan accounts and push and belongs to Rebl

#### Scenario: A release of another repository
- **WHEN** an agent runs `multica issue create` with the worklane `main` of appboypov/rebl-admin and the release v1.0.1
- **THEN** it fails saying v1.0.1 is not a release of appboypov/rebl-admin, and creates no issue

#### Scenario: A release without a worklane
- **WHEN** an agent runs `multica issue create` with `--release v1.0.1` and no worklane
- **THEN** it fails saying a release needs a worklane, and creates no issue

#### Scenario: Moved by update
- **GIVEN** REBL-24 is in Fan accounts and push and v1.1.0 Account and profile
- **WHEN** an agent runs `multica issue update REBL-24` with No release
- **THEN** REBL-24 shows under "No release" on the Timeline of Fan accounts and push
