## ADDED Requirements

### Requirement: Answers never start a run
Sending, partly sending or editing an answer MUST NOT queue or start any agent run; only a person pressing **Wake** SHALL start one.

#### Scenario: Brian answers and nothing starts
- **GIVEN** Pieter asked Brian a question on WORK-412, assigned to Pieter
- **WHEN** Brian sends his answer
- **THEN** no run is queued for Pieter or any other agent

### Requirement: Wake the asking agent from the answered card
Once a question has an answer its asking agent has not seen, its answered card SHALL offer **Wake** for that agent, named after it, such as **Wake Pieter**.

#### Scenario: Wake offered after the answer
- **GIVEN** Brian answered Pieter's question on WORK-412 and Pieter has had no run since
- **WHEN** Brian looks at the folded card
- **THEN** it offers **Wake Pieter**

#### Scenario: No Wake once the agent saw the answer
- **GIVEN** Pieter was woken after Brian's answer and no new answer has been given since
- **WHEN** Brian looks at the folded card
- **THEN** it offers no **Wake Pieter**

### Requirement: A woken run gets the answers it has not seen
A run started by **Wake** SHALL start with every answer given since that agent's last run on the question's links, and the issue's timeline SHALL record who woke whom with how many answers.

#### Scenario: Brian wakes Pieter
- **GIVEN** Brian answered Pieter's question on WORK-412 with "per line"
- **WHEN** Brian presses **Wake Pieter**
- **THEN** a new run of Pieter on WORK-412 starts with the answer "per line" in its input
- **AND** the timeline reads "Pieter was woken by Brian with 1 answer"
