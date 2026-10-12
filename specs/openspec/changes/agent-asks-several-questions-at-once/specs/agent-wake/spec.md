## ADDED Requirements

### Requirement: Wake from a question set's footer
Once any question of a set has an answer its asking agent has not seen, the set's footer SHALL offer **Wake** for that agent, also while other questions of the set are open, and the woken run SHALL get every answer of the set given since that agent's last run.

#### Scenario: Wake after the first answer
- **GIVEN** a question set of three for Brian on WORK-412, asked by Pieter
- **WHEN** Brian sends the answer to the first question
- **THEN** the set's footer offers **Wake Pieter**
- **AND** no run is queued for Pieter

#### Scenario: Waking with two of three answers
- **GIVEN** Brian answered two of the three questions of Pieter's set and Pieter has had no run since
- **WHEN** Brian presses **Wake Pieter** in the set's footer
- **THEN** Pieter's new run starts with both answers
- **AND** the timeline reads "Pieter was woken by Brian with 2 answers"
