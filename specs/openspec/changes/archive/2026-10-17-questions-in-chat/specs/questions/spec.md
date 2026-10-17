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

### Requirement: Question lifecycle
A question SHALL be open until answered, answered once its answer is sent, and closed only when the asker closes it; the asker SHALL be able to withdraw an open or answered question, a current answerer SHALL be able to cancel an open or partly answered question, and a withdrawn or cancelled question MUST leave every open list.

#### Scenario: The asker closes an answered question
- **GIVEN** Brian answered Pieter's question
- **WHEN** Pieter closes it
- **THEN** the question is closed and its answer can no longer be edited

#### Scenario: The asker withdraws a question
- **GIVEN** Pieter's question on WORK-412 is open
- **WHEN** Pieter withdraws it
- **THEN** it no longer counts as open on WORK-412 and no longer shows as waiting for Brian

#### Scenario: An answerer cancels a question
- **GIVEN** Pieter's question on WORK-412 is open for Brian
- **WHEN** Brian cancels it
- **THEN** it is cancelled, no longer counts as open on WORK-412 and no longer shows as waiting for Brian
