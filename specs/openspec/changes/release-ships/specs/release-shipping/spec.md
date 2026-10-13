## ADDED Requirements

### Requirement: The next release ships once its issues are done and merged
Multica SHALL ship a repository's next release, the first unshipped release on its line, as soon as it holds at least one issue, every issue in it is Done, and every pull request linked to those issues is merged into the repository's default branch; an issue without a linked pull request SHALL count once it is Done. Multica MUST NOT ship a release that is not the next release or holds no issue, and MUST NOT ship anything for a repository that does not release.

#### Scenario: The last pull request merges
- **GIVEN** v1.0.0 First production release, the next release of appboypov/rebl, holds REBL-22 (Done, its pull request merged into `main`) and REBL-76 (In Review) with its pull request open
- **WHEN** that pull request is merged into `main` and Multica moves REBL-76 to Done
- **THEN** v1.0.0 First production release ships

#### Scenario: Done without a pull request
- **GIVEN** v1.0.0 First production release holds REBL-22 (Done, its pull request merged into `main`) and REBL-21 (In Progress) without a linked pull request
- **WHEN** Brian drags REBL-21 to Done on the Board of Fan accounts and push
- **THEN** v1.0.0 First production release ships

#### Scenario: A linked pull request is still open
- **GIVEN** v1.0.0 First production release holds only REBL-21, whose linked pull request is open
- **WHEN** Brian drags REBL-21 to Done on the Board of Fan accounts and push
- **THEN** v1.0.0 stays Next release and does not ship
- **AND** once that pull request is merged into `main`, v1.0.0 ships

#### Scenario: A release without issues
- **GIVEN** v1.0.1 Membership hardening, the next release, holds no issue, and v1.1.0 Account and profile after it holds only REBL-24, Done without a linked pull request
- **WHEN** Brian opens the Timeline of Fan accounts and push
- **THEN** v1.0.1 Membership hardening is Next release and v1.1.0 Account and profile is Planned
- **AND** GitHub has no tag v1.0.1 or v1.1.0

#### Scenario: The release after it is already done
- **GIVEN** v1.0.0 First production release holds only REBL-21 (Todo), and v1.0.1 Membership hardening after it holds only REBL-22 (Done, its pull request merged into `main`)
- **WHEN** Brian drags REBL-21 to Done on the Board of Fan accounts and push
- **THEN** v1.0.0 First production release ships and then v1.0.1 Membership hardening ships

#### Scenario: A repository that does not release
- **GIVEN** appboypov/rebl-site is set to "This repository does not release"
- **WHEN** every issue of its worklane `main` reaches Done
- **THEN** Multica makes no commit, tag or GitHub release on appboypov/rebl-site

### Requirement: Shipping commits the version and tags it
Shipping SHALL write the release's version, without its `v`, into the version file of the repository's version policy on the default branch, changing only the version and keeping a build number after `+`, commit that change to the default branch through the GitHub App, and tag that commit `v{version}`. For a repository with "Tags only" it SHALL make no commit and tag the default branch's latest commit `v{version}`.

#### Scenario: pubspec.yaml with a build number
- **GIVEN** appboypov/rebl has "Version in pubspec.yaml" and its `pubspec.yaml` on `main` holds `version: 0.9.0+14`
- **WHEN** v1.0.0 First production release ships
- **THEN** `main` has a new commit by Multica's GitHub App that changes that line to `version: 1.0.0+14` and nothing else
- **AND** the tag v1.0.0 points at that commit

#### Scenario: package.json
- **GIVEN** appboypov/rebl-admin has "Version in package.json" and its `package.json` on `main` holds `"version": "0.4.2"`
- **WHEN** its release v0.4.3 ships
- **THEN** `main` has a new commit that sets `"version": "0.4.3"`, tagged v0.4.3

#### Scenario: Tags only
- **GIVEN** appboypov/rebl-site is set to "Tags only" with the last tag v2.1.0, and its release v2.1.1 holds only Done issues
- **WHEN** v2.1.1 ships
- **THEN** `main` gets no new commit and the tag v2.1.1 points at its latest commit

### Requirement: Shipping publishes the GitHub release
Shipping SHALL publish a GitHub release for the new tag, named with the release's title, with the release's description as its note; a release without a description SHALL get a note with one line per issue of the release, each with its type, ID and title.

#### Scenario: With a description
- **GIVEN** v1.0.0 First production release has the description "The fan app in the stores"
- **WHEN** it ships
- **THEN** appboypov/rebl has the GitHub release v1.0.0 named "First production release" with the note "The fan app in the stores"

#### Scenario: Without a description
- **GIVEN** v1.0.1 Membership hardening has no description and holds REBL-22 Fan registers and confirms their email (Story) and REBL-76 E2E Account and profile (Chore)
- **WHEN** it ships
- **THEN** the GitHub release v1.0.1 named "Membership hardening" has a note with the lines "Story REBL-22 Fan registers and confirms their email" and "Chore REBL-76 E2E Account and profile"

### Requirement: A shipped release gets its ship date and the next release moves on
A release SHALL get the date it shipped as its ship date, and the first unshipped release after it SHALL become Next release on every worklane of the repository.

#### Scenario: v1.0.0 ships
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release (Next release) and v1.0.1 Membership hardening (Planned)
- **WHEN** v1.0.0 ships on 14 October 2026
- **THEN** v1.0.0 has the ship date 14 Oct 2026
- **AND** v1.0.1 Membership hardening is Next release on the Timelines of `main`, Fan accounts and push and Agenda and publication

### Requirement: Every worklane's activity records a shipped release
Shipping SHALL add the entry "Multica shipped {version} {title}" with its time to the activity of every worklane of the repository.

#### Scenario: v1.0.0 ships
- **WHEN** v1.0.0 First production release ships at 10:05 on 14 October 2026
- **THEN** the activity of `main`, Fan accounts and push and Agenda and publication shows "Multica shipped v1.0.0 First production release" at 10:05 on 14 October 2026
