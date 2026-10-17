# release-page Specification

## Purpose
Gives every release its own page with its issues in order, its owner and its discussion.

## Requirements

### Requirement: Every release has its own page
Every release SHALL have its own page on web and desktop showing its repository, version, title, state ("Next release", "Planned" or shipped with its date), description and owner, or "No owner" while it has none.

#### Scenario: A planned release with an owner
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release and then v1.1.0 Account and profile, owned by Pieter, with the description "Fans make an account, sign in and keep their own profile with a photo."
- **WHEN** Brian opens the page of v1.1.0
- **THEN** it shows appboypov/rebl, v1.1.0, Account and profile, "Planned", that description and Pieter as owner

#### Scenario: A release without an owner
- **WHEN** Brian opens the page of v1.0.1 Membership hardening, which has no owner
- **THEN** it shows "No owner"

### Requirement: A release page lists the release's issues in order
A release page SHALL list every issue in the release in its order on the release, each with its key, title, status, assignee and worklane, and clicking an issue SHALL open its issue page. A release without issues SHALL show "No issues in this release yet."

#### Scenario: Two issues
- **GIVEN** v1.1.0 Account and profile holds WORK-501 Sign-in (in progress, Joeri, Fan accounts and push) and then WORK-502 Profile page (backlog, Anouk, Fan accounts and push)
- **WHEN** Brian opens the page of v1.1.0
- **THEN** it lists WORK-501 before WORK-502, each with its status, assignee and worklane
- **AND** clicking WORK-502 opens WORK-502's issue page

#### Scenario: No issues
- **WHEN** Brian opens the page of v1.0.1 Membership hardening, which holds no issue
- **THEN** it shows "No issues in this release yet."

### Requirement: A release page sets the owner
The owner on an unshipped release's page SHALL open the same Owner picker as Edit release, and picking SHALL set, change or remove the owner with the same effects as Edit release. A shipped release's page SHALL show its owner without a picker.

#### Scenario: Owner picked on the page
- **GIVEN** v1.1.0 Account and profile has no owner
- **WHEN** Brian clicks "No owner" on its page and picks Pieter
- **THEN** the page shows Pieter as owner
- **AND** a coordination run of Pieter on v1.1.0 is queued

#### Scenario: A shipped release
- **GIVEN** v1.0.0 First production release is shipped with Pieter as owner
- **WHEN** Brian opens its page
- **THEN** it shows Pieter as owner and offers no Owner picker

### Requirement: A release page holds comments
A release page SHALL show the release's comments and their replies oldest first, each with its author and time, mixed in time order with the release's owner changes, and SHALL offer a comment box and a reply on each comment, as an issue page does.

#### Scenario: Brian comments
- **WHEN** Brian writes "Start WORK-502 once WORK-501's pull request is open." in the comment box of v1.1.0 Account and profile and sends it
- **THEN** the page shows that comment with Brian as author and its time

#### Scenario: Comments and owner changes in order
- **GIVEN** Brian made Pieter owner of v1.1.0 Account and profile at 10:05 and Pieter commented at 10:12
- **WHEN** Brian opens the page of v1.1.0
- **THEN** it shows "Brian made Pieter owner of v1.1.0 Account and profile" before Pieter's comment

### Requirement: A release's title on the Timeline opens its page
A release's version and title on a worklane's Timeline SHALL open the release's page.

#### Scenario: From the Timeline
- **WHEN** Brian clicks v1.1.0 Account and profile on the Timeline of Fan accounts and push
- **THEN** the page of v1.1.0 Account and profile opens

### Requirement: A release page leads to the Timeline
A release page SHALL offer "Open on Timeline", which opens the Timeline of the first worklane of the repository, in its order, that holds issues in the release, else of `main`, with that release in view.

#### Scenario: Issues in Fan accounts and push
- **GIVEN** v1.1.0 Account and profile holds only issues of Fan accounts and push
- **WHEN** Brian clicks "Open on Timeline" on its page
- **THEN** the Timeline of Fan accounts and push opens with v1.1.0 in view

#### Scenario: A release without issues
- **GIVEN** v1.0.1 Membership hardening holds no issue
- **WHEN** Brian clicks "Open on Timeline" on its page
- **THEN** the Timeline of `main` of appboypov/rebl opens with v1.0.1 in view
