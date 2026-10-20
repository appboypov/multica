## ADDED Requirements

### Requirement: A repository says whether it has a beta environment
Edit repository SHALL offer the switch "Beta environment", off for every repository until it is turned on, and a release SHALL have a beta phase only while its repository has a beta environment.

#### Scenario: MultiChannable has beta
- **GIVEN** MultiChannable/multichannable has no beta environment set
- **WHEN** Brian turns on "Beta environment" in Edit repository of MultiChannable/multichannable and saves
- **THEN** the page of v0.22.3 Sync security shows the phases On main, Passed on staging, Beta and Production

#### Scenario: Rebl has no beta
- **GIVEN** appboypov/rebl has "Beta environment" off
- **WHEN** Brian opens the page of v1.1.0 Account and profile
- **THEN** it shows the phases On main, Passed on staging and Production, and no Beta

### Requirement: Getting on main is recorded and starts no deployment
A release SHALL be on `main` only once that is recorded for it, and recording it SHALL be refused with "every issue must be Done and every linked pull request merged before it goes on main" while an issue of the release is not Done or a linked pull request is not merged. Recording it MUST NOT start a deployment and MUST NOT record a staging pass or a consent.

#### Scenario: Every issue already on main
- **GIVEN** every issue of v1.1.0 Account and profile of appboypov/rebl is Done and its pull requests are merged into `main`
- **WHEN** v1.1.0 is recorded as on main
- **THEN** its page shows On main with the date and who recorded it
- **AND** Passed on staging shows "Needs a staging pass" and Production shows "Needs a staging pass"

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
- **GIVEN** v1.0.1 Membership hardening is not shipped and v1.1.0 Account and profile has its staging pass and its consent for production
- **WHEN** the production start of v1.1.0 is recorded
- **THEN** it is refused with "v1.0.1 Membership hardening is not in production yet."

### Requirement: A staging pass is one click of an owner or admin
The release page of a release on `main` SHALL offer "Passed on staging" to a workspace owner or admin, and clicking it SHALL record the staging pass with who gave it and when, without a link, a commit or any other field. Recording a staging pass SHALL be refused with "Get {version} on main first." for a release that is not on `main`.

#### Scenario: Passed on staging
- **GIVEN** v0.22.3 Sync security of MultiChannable/multichannable is on main
- **WHEN** Brian clicks "Passed on staging" on its page
- **THEN** the page shows Passed on staging with Brian and the date

#### Scenario: Not on main yet
- **GIVEN** v0.22.3 Sync security is not on main
- **WHEN** Brian opens its page
- **THEN** it offers no "Passed on staging"
- **AND** a staging pass recorded for v0.22.3 through the API is refused with "Get v0.22.3 on main first."

### Requirement: Only an owner or admin records a staging pass or a consent
A staging pass and a consent SHALL be recorded only by a workspace owner or admin, and Multica SHALL refuse one recorded by an agent, a squad's run, a client or a member without that role with "Only an owner or admin can record a pass or a consent."

#### Scenario: An agent tries to give consent
- **GIVEN** Pieter, an agent, owns v0.22.3 Sync security, which has its staging pass
- **WHEN** Pieter's run records a consent for beta of v0.22.3
- **THEN** it is refused with "Only an owner or admin can record a pass or a consent."
- **AND** Beta still shows "Needs consent"

### Requirement: Each deployment needs its own consent
After the staging pass, the release page SHALL offer "Consent for beta" while the repository has a beta environment and "Consent for production" to a workspace owner or admin, and clicking one SHALL record the consent for that phase only, with who gave it and when. A consent for beta MUST NOT count for production. A consent SHALL be refused with "Pass {version} on staging first." before the staging pass, and a consent for beta with "{repository} has no beta environment." when the repository has none.

#### Scenario: Brian consents to beta
- **GIVEN** v0.22.3 Sync security has its staging pass
- **WHEN** Brian clicks "Consent for beta" on its page
- **THEN** Beta shows "Ready to start" with Brian and the date
- **AND** Production still shows "Needs consent"

#### Scenario: No beta environment
- **GIVEN** appboypov/rebl has "Beta environment" off and v1.1.0 Account and profile has its staging pass
- **WHEN** a consent for beta of v1.1.0 is recorded through the API
- **THEN** it is refused with "appboypov/rebl has no beta environment."

### Requirement: A consent wakes the release's owner to deploy
Recording a consent SHALL queue one coordination run of the release's owner whose input names the phase it may now start, and SHALL queue nothing when the release has no owner. Getting on `main`, a staging pass and a start MUST NOT queue a run that names a phase to start.

#### Scenario: Pieter gets to deploy beta
- **GIVEN** Pieter owns v0.22.3 Sync security, which has its staging pass
- **WHEN** Brian clicks "Consent for beta" on its page
- **THEN** a coordination run of Pieter on v0.22.3 is queued whose input says beta may now start

#### Scenario: No owner
- **GIVEN** v1.1.0 Account and profile has no owner and has its staging pass
- **WHEN** Brian clicks "Consent for production" on its page
- **THEN** no run is queued and Production shows "Ready to start"

### Requirement: Nothing records a consent by itself
Getting on `main`, a passed check or test, a staging pass, a beta start, a coordination run and an approval of the release pull request MUST NOT record a consent, and the release page SHALL keep showing the consent each phase still needs until that consent itself is recorded.

#### Scenario: Beta started, production still needs consent
- **GIVEN** v0.22.3 Sync security is on main, passed on staging and its beta is started
- **WHEN** Brian opens its page
- **THEN** Production shows "Needs consent"

### Requirement: Beta starts by hand only after its staging pass and its consent
A beta start SHALL be recorded only by hand with `multica release start <release> --phase beta`, by an agent or a member, and SHALL be refused with "Pass {version} on staging first." without a staging pass and with "{version} has no consent for beta." without a consent for beta. A beta start MUST NOT start production.

#### Scenario: Beta started
- **GIVEN** v0.22.3 Sync security has its staging pass and Brian's consent for beta
- **WHEN** Pieter runs `multica release start` for v0.22.3 with the phase beta
- **THEN** Beta shows started with Pieter and the date
- **AND** Production shows "Needs consent"

#### Scenario: No consent yet
- **GIVEN** v0.22.3 Sync security has its staging pass and no consent for beta
- **WHEN** Pieter runs `multica release start` for v0.22.3 with the phase beta
- **THEN** it is refused with "v0.22.3 has no consent for beta."

### Requirement: Production starts by hand only after its staging pass, its beta and its own consent
A production start SHALL be recorded only by hand with `multica release start <release> --phase production`, by an agent or a member, and SHALL be refused with "Pass {version} on staging first." without a staging pass, with "Start beta first." when the repository has a beta environment and the release's beta is not started, and with "{version} has no consent for production." without a consent for production. A repository without a beta environment SHALL skip only the beta phase.

#### Scenario: Beta not started
- **GIVEN** MultiChannable/multichannable has a beta environment and v0.22.3 Sync security has its staging pass and a consent for production but no beta start
- **WHEN** Pieter runs `multica release start` for v0.22.3 with the phase production
- **THEN** it is refused with "Start beta first."

#### Scenario: Rebl goes to production without beta
- **GIVEN** appboypov/rebl has no beta environment and v1.1.0 Account and profile has its staging pass and Brian's consent for production
- **WHEN** Pieter runs `multica release start` for v1.1.0 with the phase production
- **THEN** Production shows started with Pieter and the date

### Requirement: Shipping needs the production start
`release ship` SHALL be refused with "Start production first." for a release without a production start, and SHALL otherwise mark the release shipped with its existing checks.

#### Scenario: Shipped after the production start
- **GIVEN** v1.1.0 Account and profile has its production start
- **WHEN** `release ship` runs for v1.1.0 once it is live
- **THEN** v1.1.0 is shipped with that date

#### Scenario: Production not started
- **GIVEN** v1.1.0 Account and profile is on main and passed on staging without a production start
- **WHEN** `release ship` runs for v1.1.0
- **THEN** it is refused with "Start production first."

### Requirement: A release on main keeps its version and its place
A release on `main` SHALL keep its version and its place on the line: changing its version, moving it, removing it, or a move or removal of another release that would change its version or put an unshipped release that is not on `main` before it SHALL be refused with "{version} {title} is on main: its version and place are fixed."

#### Scenario: Moving a planned release before it
- **GIVEN** appboypov/rebl's line holds v1.1.0 Account and profile, on main, and then v1.2.0 Notifications, planned
- **WHEN** Brian chooses Move earlier on v1.2.0 Notifications
- **THEN** it is refused with "v1.1.0 Account and profile is on main: its version and place are fixed."
- **AND** the line reads v1.1.0 Account and profile, v1.2.0 Notifications
