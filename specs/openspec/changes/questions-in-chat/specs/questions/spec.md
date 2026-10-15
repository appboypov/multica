## MODIFIED Requirements

### Requirement: A question asked during a run links to where it was asked
A question asked during an issue run SHALL link to that run and to the run's issue, a question asked during a chat turn SHALL link to that run and to the chat, and its first link SHALL be where it was asked.

#### Scenario: Links of a question asked in run #3
- **GIVEN** Pieter is in run #3 on WORK-412
- **WHEN** Pieter asks a question
- **THEN** the question links to WORK-412 and to run #3
- **AND** WORK-412 is its first link

#### Scenario: Links of a question asked in a chat
- **GIVEN** Pieter is in a turn of Brian's chat
- **WHEN** Pieter asks a question
- **THEN** the question links to Brian's chat and to the turn's run
- **AND** Brian's chat is its first link
