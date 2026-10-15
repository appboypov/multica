## MODIFIED Requirements

### Requirement: Answers never start a run
Sending, partly sending or editing an answer MUST NOT queue or start any agent run, except the answer that leaves no question of an ask made in a chat open, which starts the asking agent's next turn in that chat; otherwise only a person pressing **Wake** SHALL start one.

#### Scenario: Brian answers and nothing starts
- **GIVEN** Pieter asked Brian a question on WORK-412, assigned to Pieter
- **WHEN** Brian sends his answer
- **THEN** no run is queued for Pieter or any other agent

#### Scenario: The last answer in a chat
- **GIVEN** Pieter asked Brian one question in Brian's chat
- **WHEN** Brian sends his answer
- **THEN** Pieter's next turn in Brian's chat is queued and no other run is queued
