## ADDED Requirements

### Requirement: A member's question wakes no agent
A question a member asked MUST NOT offer **Wake** anywhere, on its card, its set's footer, beside Send or in Answer all, and its answers, answer edits and ask backs MUST NOT queue any agent run or reach a running run as input.

#### Scenario: Brian answers Skuddy's last question
- **GIVEN** Skuddy's omp session asked Brian one question on WORK-442, which is assigned to the agent Joeri
- **WHEN** Brian opens the card and sends his answer
- **THEN** no **Wake** checkbox is shown beside Send
- **AND** no run is queued for Joeri or any other agent

#### Scenario: Brian asks back on Skuddy's question
- **GIVEN** Skuddy's omp session asked Brian a question on WORK-442
- **WHEN** Brian asks back "Which plan do you mean?"
- **THEN** no run is queued for any agent

#### Scenario: Joeri's running run hears nothing
- **GIVEN** Joeri's run on WORK-442 is running and Skuddy's omp session asked Brian a question on WORK-442
- **WHEN** Brian answers it
- **THEN** Joeri's run receives no input about the answer

#### Scenario: The answered card
- **GIVEN** Brian answered Skuddy's question on WORK-442, which is assigned to the agent Joeri
- **WHEN** Brian looks at the folded card and at Answer all
- **THEN** neither offers **Wake**

#### Scenario: Pieter's question still wakes him
- **GIVEN** Pieter asked Brian one question in a run on WORK-412 and that run has ended
- **WHEN** Brian sends his answer with **Wake Pieter** checked
- **THEN** a new run of Pieter on WORK-412 starts with the answer in its input
