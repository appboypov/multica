## ADDED Requirements

### Requirement: Confident routine choices
The server SHALL propose configured Jev choices only when action and reason confidence meet the workspace threshold. It SHALL wait for Brian's answer during the start period and apply choices automatically after Brian ends that period.

#### Scenario: Confident bug choice
- **GIVEN** a matching post-merge bug and a configured kind lead
- **AND** Brian has ended the start period
- **WHEN** Jev selects fix_now and a reason at confidence 0.95
- **THEN** the issue is todo, assigned to the lead and has a Jev audit comment

### Requirement: Uncertain choices use the Decider
The server SHALL queue a Sonnet 5.5 Decider task for a low-confidence choice and route its valid answer through the same confirmation and action path, retaining Jev's confidence.

#### Scenario: Uncertain bug choice
- **GIVEN** a matching bug and confidence below 0.85
- **WHEN** Jev answers
- **THEN** a Decider run receives the question, options and issue context without applying Jev's choice

#### Scenario: Decider completion
- **GIVEN** a pending decision task
- **WHEN** it returns a valid choice and reason for the same issue state
- **THEN** the choice follows the workspace confirmation policy and an issue comment names the Decider agent and Jev confidence

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

### Requirement: Brian controls the start period
The workspace SHALL start with every choice waiting for Brian's answer and a confidence threshold of 0.85. Only Brian SHALL end the start period or change the threshold. Policy SHALL never change by itself.

#### Scenario: Brian confirms or corrects a proposal
- **GIVEN** the start period is on and Jev or Sonnet proposes a choice
- **WHEN** Brian selects the proposed choice or the other choice in its question to-do
- **THEN** only his selected action is applied and the issue records agreed or corrected, the maker, Jev confidence and his note

#### Scenario: Another actor changes policy
- **GIVEN** Brian is the configured member
- **WHEN** an agent or another member changes or removes the decider settings
- **THEN** the server refuses the write

### Requirement: Deterministic samples and corrections
After the start period the server SHALL send every fifth automatic choice in the workspace to Brian after applying it. His answer SHALL be recorded with the same agreement fields. A correction SHALL apply his selected outcome and reconcile this decision's task and to-dos.

#### Scenario: Fifth automatic choice
- **GIVEN** four automatic choices have committed in the workspace
- **WHEN** the next automatic choice commits
- **THEN** it is applied and Brian gets its confirmation question, while choices one through four have none

#### Scenario: Brian corrects an applied fix-now
- **GIVEN** an automatic fix-now choice opened a lead task
- **WHEN** Brian selects fix-later on its sample question
- **THEN** the issue returns to backlog, this decision's active lead task is cancelled and Brian gets the planning to-do

### Requirement: Durable agreement record
The server SHALL append every answer as agreed or corrected with the original Jev confidence, maker and selected choice. A repeated identical answer SHALL have no duplicate effect. Answer edits SHALL retain earlier answers.

#### Scenario: Brian edits an answer
- **GIVEN** Brian answered a routine question
- **WHEN** he selects a different choice
- **THEN** the selected outcome is reconciled and both answers remain in the receipt and issue comments
