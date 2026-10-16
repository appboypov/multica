## MODIFIED Requirements

### Requirement: An answer reaches the running runs on the question's issues
When a person sends or changes an answer, or asks back, on a question linked to an issue, each running run on that issue SHALL receive the answer or ask-back as a notice, and no run SHALL be queued for it while that run runs.

#### Scenario: Brian answers while Thomas runs
- **GIVEN** Thomas asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian sends the answer "use the toolbar copy button"
- **THEN** Thomas's run receives "use the toolbar copy button", named as Brian's answer to Thomas's question
- **AND** no run is queued for Thomas

#### Scenario: Brian answers Skuddy while Thomas runs
- **GIVEN** Skuddy asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian answers it
- **THEN** Thomas's run receives the answer, named as Brian's answer to Skuddy's question

#### Scenario: Brian changes his answer while Thomas runs
- **GIVEN** Thomas's running run on WORK-434 received Brian's answer "per line"
- **WHEN** Brian changes the answer to "per total"
- **THEN** Thomas's run receives "per total", named as Brian's changed answer

#### Scenario: Brian asks back while Thomas runs
- **GIVEN** Thomas asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian asks back "which toolbar?"
- **THEN** Thomas's run receives "which toolbar?", named as Brian's ask-back
