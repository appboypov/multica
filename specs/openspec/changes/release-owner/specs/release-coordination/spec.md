## ADDED Requirements

### Requirement: Making an owner starts a coordination run
Setting or changing the owner of an unshipped release SHALL queue one coordination run on the release: for an agent owner a run of that agent, for a squad owner a run of the squad's leader with the squad's briefing, under the readiness rules of issue assignment. No other squad member SHALL get a run from it.

#### Scenario: Agent owner
- **WHEN** Brian makes Pieter owner of v1.1.0 Account and profile
- **THEN** a coordination run of Pieter on v1.1.0 is queued

#### Scenario: Squad owner
- **GIVEN** Build squad is led by Joeri and has the member Anouk
- **WHEN** Brian makes Build squad owner of v1.1.0 Account and profile
- **THEN** a coordination run of Joeri on v1.1.0 is queued with Build squad's roster and instructions in its input
- **AND** no run of Anouk is queued

#### Scenario: Agent without a runtime
- **GIVEN** the agent Kees has no runtime
- **WHEN** Brian makes Kees owner of v1.1.0 Account and profile
- **THEN** Kees owns v1.1.0 and no run is queued

### Requirement: Removing or changing the owner leaves a started run alone
Removing or changing a release's owner MUST NOT cancel a coordination run that already started, and a removed owner SHALL get no further coordination runs on the release.

#### Scenario: Owner removed while running
- **GIVEN** Pieter's coordination run on v1.1.0 Account and profile is running
- **WHEN** Brian removes Pieter as owner
- **THEN** Pieter's run goes on until it ends
- **AND** when WORK-501 in v1.1.0 is done afterwards, no run of Pieter is queued

### Requirement: A coordination run receives the release and its issues
A coordination run's input SHALL hold the release's repository, version, title and description, and every issue in the release in its order on the release with its key, title, status, assignee, parent, worklane and linked pull requests, and the reason the run started: the owner change, the issue that ended, the wait that fired, or the comment on the release page that woke it.

#### Scenario: First run after assignment
- **GIVEN** v1.1.0 Account and profile on appboypov/rebl holds WORK-501 Sign-in (in progress, Joeri, worklane Fan accounts and push) and then WORK-502 Profile page (backlog, Anouk, worklane Fan accounts and push)
- **WHEN** Pieter's coordination run starts after Brian made him owner
- **THEN** its input names appboypov/rebl, v1.1.0 Account and profile and its description
- **AND** lists WORK-501 before WORK-502, each with its status, assignee, parent, worklane and linked pull requests
- **AND** says Brian made Pieter owner

### Requirement: A coordination run keeps Brian's release
A coordination run's input SHALL state that the release's issues, their order, their assignees, parents and worklanes, and the release's version and place on the line stay as Brian set them, and that the owner carries the release through the release workflow with every gate of Brian's and ships only through `release ship`.

#### Scenario: The brief in the input
- **WHEN** Pieter's coordination run on v1.1.0 Account and profile starts
- **THEN** its input says the issues of v1.1.0, their order, assignees, parents and worklanes, and v1.1.0's version and place stay as Brian set them
- **AND** says v1.1.0 goes through the release workflow with Brian's gates and ships only through `release ship`

### Requirement: A member's comment on the release page wakes its owner
A comment or reply by a member on the page of a release with an owner SHALL queue a coordination run of the owner, a squad's leader for a squad, whose input holds that comment; when the owner already has a queued coordination run on the release, the comment SHALL ride along in that run instead of queueing another. A comment on a release without an owner MUST NOT queue a run.

#### Scenario: Brian comments
- **GIVEN** Pieter owns v1.1.0 Account and profile and has no queued run on it
- **WHEN** Brian comments "Start WORK-502 once WORK-501's pull request is open." on v1.1.0's page
- **THEN** a coordination run of Pieter on v1.1.0 is queued whose input holds that comment

#### Scenario: Two comments before the run starts
- **GIVEN** Pieter owns v1.1.0 Account and profile and Brian's first comment queued Pieter's run
- **WHEN** Brian comments again before that run starts
- **THEN** Pieter still has one queued coordination run on v1.1.0

#### Scenario: No owner
- **WHEN** Brian comments on the page of v1.0.1 Membership hardening, which has no owner
- **THEN** the comment shows and no run is queued

### Requirement: A coordination run reports on the release page
A coordination run SHALL post its result as a comment on its release's page, and the owner SHALL be able to read and add comments on its release with `multica release comment list` and `multica release comment add`, replies included.

#### Scenario: Pieter reports
- **GIVEN** Pieter's coordination run on v1.1.0 Account and profile moved WORK-502 out of backlog
- **WHEN** the run ends
- **THEN** v1.1.0's page shows Pieter's comment with the run's result

#### Scenario: Reply to Brian
- **GIVEN** Brian's comment on v1.1.0's page woke Pieter
- **WHEN** Pieter's run replies with `multica release comment add` to that comment
- **THEN** v1.1.0's page shows Pieter's reply under Brian's comment

### Requirement: The owner wakes when an issue of its release ends
When an issue in a release with an owner becomes done or cancelled, the platform SHALL queue a coordination run of the owner, a squad's leader for a squad, whose input names that issue, how it ended, and the issues still open in order; when the last open issue becomes done and every issue is done, the run's input SHALL say every issue of the release is done.

#### Scenario: One issue done
- **GIVEN** Pieter owns v1.1.0 Account and profile, which holds WORK-501 Sign-in and WORK-502 Profile page
- **WHEN** WORK-501 becomes done
- **THEN** a coordination run of Pieter on v1.1.0 is queued
- **AND** its input says WORK-501 is done and WORK-502 is still open

#### Scenario: One issue cancelled
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** WORK-502 Profile page is cancelled
- **THEN** a coordination run of Pieter on v1.1.0 is queued whose input says WORK-502 was cancelled

#### Scenario: Every issue done
- **GIVEN** Pieter owns v1.1.0 Account and profile and WORK-502 is its only open issue
- **WHEN** WORK-502 becomes done
- **THEN** Pieter's queued coordination run says every issue of v1.1.0 is done

#### Scenario: A release without an owner
- **GIVEN** v1.0.1 Membership hardening has no owner
- **WHEN** WORK-480 in v1.0.1 becomes done
- **THEN** no coordination run is queued

### Requirement: The owner sets its own waits on its release
The owner SHALL be able to create, list and delete wakeups on its release with `multica release wakeup`, which queue a coordination run of the release's owner of that moment when an issue of the workspace reaches done, ended or in_review, at a time, every interval or on a cron schedule, with the expiry, timeout and instruction options of issue wakeups.

#### Scenario: Wait for review
- **GIVEN** Pieter owns v1.1.0 Account and profile and created a release wakeup on v1.1.0 until WORK-501 is in_review
- **WHEN** WORK-501 moves to in_review
- **THEN** a coordination run of Pieter on v1.1.0 is queued whose input names that wakeup and WORK-501

### Requirement: Coordination never ships a release
The platform MUST NOT mark a release shipped because its issues are done or because of a coordination run; a release SHALL become shipped only through `release ship` with its existing checks.

#### Scenario: Every issue done
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** every issue of v1.1.0 is done and its pull requests are merged
- **THEN** v1.1.0 stays unshipped until `release ship` runs for it

### Requirement: Removing or shipping a release ends its coordination
Removing a release SHALL cancel its queued coordination runs and delete its wakeups; shipping a release SHALL delete its wakeups, and a shipped release SHALL get no coordination run.

#### Scenario: Removed
- **GIVEN** Pieter owns v1.2.0 Notifications, has a queued coordination run on it and a wakeup until WORK-510 is done
- **WHEN** Brian removes v1.2.0
- **THEN** Pieter's queued run on v1.2.0 is cancelled
- **AND** when WORK-510 becomes done, no run of Pieter is queued

#### Scenario: Shipped
- **GIVEN** Pieter owns v1.1.0 Account and profile and has a wakeup on it every day at 09:00
- **WHEN** v1.1.0 ships
- **THEN** the wakeup is gone and no coordination run of Pieter on v1.1.0 is queued at 09:00
