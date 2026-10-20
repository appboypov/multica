## ADDED Requirements

### Requirement: A repository says whether it has a beta environment
Edit repository SHALL offer the switch "Beta environment", off for every repository until it is turned on, and a release SHALL have a beta phase only while its repository has a beta environment.

#### Scenario: MultiChannable has beta
- **GIVEN** MultiChannable/multichannable has no beta environment set
- **WHEN** Brian turns on "Beta environment" in Edit repository of MultiChannable/multichannable and saves
- **THEN** the page of v0.22.3 Sync security shows the phases On main, Tested on staging, Beta and Production

#### Scenario: Rebl has no beta
- **GIVEN** appboypov/rebl has "Beta environment" off
- **WHEN** Brian opens the page of v1.1.0 Account and profile
- **THEN** it shows the phases On main, Tested on staging and Production, and no Beta

### Requirement: Getting on main is recorded and starts no deployment
A release SHALL be on `main` only once that is recorded for it, and recording it SHALL be refused with "every issue must be Done and every linked pull request merged before it goes on main" while an issue of the release is not Done or a linked pull request is not merged. Recording it MUST NOT start a deployment and MUST NOT record a staging test or a consent.

#### Scenario: Every issue already on main
- **GIVEN** every issue of v1.1.0 Account and profile of appboypov/rebl is Done and its pull requests are merged into `main`
- **WHEN** v1.1.0 is recorded as on main
- **THEN** its page shows On main with the date and who recorded it
- **AND** Tested on staging shows "Needs a staging test" and Production shows "Needs a staging test"

#### Scenario: An issue still open
- **GIVEN** WORK-501 of v1.1.0 Account and profile is In Review
- **WHEN** v1.1.0 is recorded as on main
- **THEN** it is refused with "every issue must be Done and every linked pull request merged before it goes on main" naming WORK-501
- **AND** v1.1.0 is not on main

### Requirement: A release reaches a phase only after the releases before it
Recording a release on main, starting its beta and starting its production SHALL each be refused with "{version} {title} is not {phase} yet." naming the first release before it on the line that has not reached that phase.

#### Scenario: The release before it is not on main
- **GIVEN** appboypov/rebl's line holds v1.0.1 Membership hardening, not on main, and then v1.1.0 Account and profile
- **WHEN** v1.1.0 is recorded as on main
- **THEN** it is refused with "v1.0.1 Membership hardening is not on main yet."

#### Scenario: The release before it is not in production
- **GIVEN** v1.0.1 Membership hardening is not shipped and v1.1.0 Account and profile has its staging test and its consent for production
- **WHEN** the production start of v1.1.0 is recorded
- **THEN** it is refused with "v1.0.1 Membership hardening is not in production yet."

### Requirement: A staging test names the commit it tested
A staging test SHALL be recorded only for a release on `main`, with the tested commit and a link to its result, and SHALL be refused with "Get {version} on main first." for a release that is not on `main`. A newer staging test of the release SHALL replace the earlier one.

#### Scenario: Tested on staging
- **GIVEN** v0.22.3 Sync security of MultiChannable/multichannable is on main
- **WHEN** a staging test of commit 24f708b with a link to its result is recorded
- **THEN** its page shows Tested on staging with commit 24f708b, the link, the date and who recorded it

#### Scenario: Not on main yet
- **GIVEN** v0.22.3 Sync security is not on main
- **WHEN** a staging test of commit 24f708b is recorded
- **THEN** it is refused with "Get v0.22.3 on main first."

### Requirement: Each deployment needs its own consent for the tested commit
A consent SHALL be recorded for one phase, beta or production, and for the commit of the release's latest staging test, naming the client or user who agreed and when. A consent for beta MUST NOT count for production, and a consent MUST NOT count once a staging test of another commit replaced the one it was given for. A consent for beta SHALL be refused with "{repository} has no beta environment." when the repository has none, and any consent SHALL be refused with "Test {version} on staging first." before the release's staging test.

#### Scenario: Milad agrees to beta
- **GIVEN** v0.22.3 Sync security has its staging test of commit 24f708b
- **WHEN** the consent of Milad for beta of commit 24f708b is recorded
- **THEN** Beta shows "Ready to start by hand" with Milad, the date and commit 24f708b
- **AND** Production still shows "Needs consent of the client or user"

#### Scenario: A new candidate
- **GIVEN** v0.22.3 has a consent for beta of commit 24f708b
- **WHEN** a staging test of commit 3a91c0e is recorded for v0.22.3
- **THEN** Beta shows "Needs consent of the client or user" for commit 3a91c0e

#### Scenario: No beta environment
- **GIVEN** appboypov/rebl has "Beta environment" off and v1.1.0 Account and profile has its staging test
- **WHEN** a consent for beta of v1.1.0 is recorded
- **THEN** it is refused with "appboypov/rebl has no beta environment."

### Requirement: Nothing records a consent by itself
Getting on `main`, a passed check or test, a staging test, a beta start, a coordination run and an approval of the release pull request MUST NOT record a consent, and the release page SHALL keep showing the consent each phase still needs until that consent itself is recorded.

#### Scenario: Beta started, production still needs consent
- **GIVEN** v0.22.3 Sync security is on main, tested on staging and its beta is started
- **WHEN** Brian opens its page
- **THEN** Production shows "Needs consent of the client or user"

### Requirement: Beta starts by hand only after its staging test and its consent
A beta start SHALL be recorded only by hand for the commit of the release's latest staging test, and SHALL be refused with "Test {version} on staging first." without a staging test and with "Record the consent of the client or user for beta first." without a consent for beta of that commit. A beta start MUST NOT start production.

#### Scenario: Beta started
- **GIVEN** v0.22.3 Sync security has its staging test of commit 24f708b and Milad's consent for beta of 24f708b
- **WHEN** the beta start of v0.22.3 is recorded
- **THEN** Beta shows started with the date, who started it and commit 24f708b

#### Scenario: No consent yet
- **GIVEN** v0.22.3 Sync security has its staging test of commit 24f708b and no consent for beta
- **WHEN** the beta start of v0.22.3 is recorded
- **THEN** it is refused with "Record the consent of the client or user for beta first."

### Requirement: Production starts by hand only after its staging test, its beta and its own consent
A production start SHALL be recorded only by hand for the commit of the release's latest staging test, and SHALL be refused with "Test {version} on staging first." without a staging test, with "Start beta first." when the repository has a beta environment and the release's beta is not started, and with "Record the consent of the client or user for production first." without a consent for production of that commit. A repository without a beta environment SHALL skip only the beta phase.

#### Scenario: Beta not started
- **GIVEN** MultiChannable/multichannable has a beta environment and v0.22.3 Sync security has its staging test and a consent for production but no beta start
- **WHEN** the production start of v0.22.3 is recorded
- **THEN** it is refused with "Start beta first."

#### Scenario: Rebl goes to production without beta
- **GIVEN** appboypov/rebl has no beta environment and v1.1.0 Account and profile has its staging test of commit 9be1d42 and Brian's consent for production of 9be1d42
- **WHEN** the production start of v1.1.0 is recorded
- **THEN** Production shows started with the date, who started it and commit 9be1d42

### Requirement: Shipping needs the production start
`release ship` SHALL be refused with "Start production first." for a release without a production start, and SHALL otherwise mark the release shipped with its existing checks.

#### Scenario: Shipped after the production start
- **GIVEN** v1.1.0 Account and profile has its production start
- **WHEN** `release ship` runs for v1.1.0 once it is live
- **THEN** v1.1.0 is shipped with that date

#### Scenario: Production not started
- **GIVEN** v1.1.0 Account and profile is on main and tested on staging without a production start
- **WHEN** `release ship` runs for v1.1.0
- **THEN** it is refused with "Start production first."

### Requirement: A release on main keeps its version and its place
A release on `main` SHALL keep its version and its place on the line: changing its version, moving it, removing it, or a move or removal of another release that would change its version or put an unshipped release that is not on `main` before it SHALL be refused with "{version} {title} is on main: its version and place are fixed."

#### Scenario: Moving a planned release before it
- **GIVEN** appboypov/rebl's line holds v1.1.0 Account and profile, on main, and then v1.2.0 Notifications, planned
- **WHEN** Brian chooses Move earlier on v1.2.0 Notifications
- **THEN** it is refused with "v1.1.0 Account and profile is on main: its version and place are fixed."
- **AND** the line reads v1.1.0 Account and profile, v1.2.0 Notifications
