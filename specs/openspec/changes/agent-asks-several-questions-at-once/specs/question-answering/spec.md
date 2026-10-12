## ADDED Requirements

### Requirement: Each answer of a set reaches the asker at once
Each answer sent to a question of a set SHALL be readable by the asker at once, without waiting for the other questions of the set, and the set's card SHALL show how many of its questions are answered.

#### Scenario: First answer of three
- **GIVEN** a question set of three open questions for Brian
- **WHEN** Brian sends the answer to the first question
- **THEN** Pieter reads that answer through `multica question` at once
- **AND** the card shows "1 of 3 answered"

#### Scenario: Leaving a set partly answered
- **GIVEN** a question set of three open questions for Brian
- **WHEN** Brian sends two answers and leaves the issue
- **THEN** the set stays open with "2 of 3 answered" and both sent answers stay sent
