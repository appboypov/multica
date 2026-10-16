## ADDED Requirements

### Requirement: Confident routine choices
The server SHALL apply configured Jev choices only when action and reason confidence meet the workspace threshold.

#### Scenario: Confident bug choice
- **GIVEN** a matching post-merge bug and a configured kind lead
- **WHEN** Jev selects fix_now and a reason at confidence 0.95
- **THEN** the issue is todo, assigned to the lead and has a Jev audit comment

### Requirement: Uncertain choices use the Decider
The server SHALL queue a Sonnet 5.5 Decider task for a low-confidence choice and apply its valid answer through the same action path.

#### Scenario: Uncertain bug choice
- **GIVEN** a matching bug and confidence below 0.85
- **WHEN** Jev answers
- **THEN** a Decider run receives the question, options and issue context without applying Jev's choice

#### Scenario: Decider completion
- **GIVEN** a pending decision task
- **WHEN** it returns a valid choice and reason for the same issue state
- **THEN** the action is applied and an issue comment names the Decider agent

### Requirement: Release membership events
The server SHALL publish a release-join event only after committed membership changes and SHALL check release readiness on joins and status changes.

#### Scenario: Issue joins a release
- **GIVEN** an issue outside a release
- **WHEN** committed placement adds it to a release
- **THEN** an issue:release_joined event names the workspace, issue and release

### Requirement: Bug and release actions
The server SHALL route fix-now to the mapped lead, fix-later to backlog and Brian's to-do inbox, and ready releases to Brian's release-check to-do inbox.

#### Scenario: Release ready
- **GIVEN** a nonempty unshipped release with every issue done
- **WHEN** the decision is ready
- **THEN** Brian gets one release-check to-do and the triggering issue has an audit comment

#### Scenario: Release not ready
- **GIVEN** a release decision
- **WHEN** the decision is not_yet
- **THEN** the issue records its reason without opening a release-check to-do

### Requirement: Auditable and scoped application
The server SHALL record the actor, reason and Jev confidence where applicable, prevent duplicate application and reject stale or cross-workspace actions.

#### Scenario: Stale answer
- **GIVEN** an issue changed while the Decider ran
- **WHEN** the answer arrives
- **THEN** it is recorded as stale and has no action
