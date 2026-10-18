## ADDED Requirements

### Requirement: Transfer the project check recipient
Opening a check for another workspace member SHALL make that member the recipient of the project and kind while preserving its record and history. Counts and sampling progress SHALL continue without reset. Existing check to-dos SHALL retain their recipients.

#### Scenario: Another member takes over sampled checks
- **GIVEN** a project and kind has answered checks and sampling in progress
- **WHEN** an agent opens its next check for another workspace member
- **THEN** the check follows the existing sampling schedule for that member
- **AND** the project record keeps its counts and previous answers
- **AND** only that member can change the record's sampling mode

#### Scenario: Earlier checks remain answerable
- **GIVEN** a member has an unanswered check
- **WHEN** an agent opens another check of the same project and kind for another workspace member
- **THEN** the earlier check remains answerable by its own recipient
- **AND** its answer contributes to the same project record

#### Scenario: A nonmember cannot take over
- **GIVEN** a project and kind has a recipient
- **WHEN** an agent tries to open its next check for a person outside the workspace
- **THEN** the request is refused and the record remains unchanged
