## ADDED Requirements

### Requirement: A wake waits for the agent's tasks on the issue
A wake of an agent that has a queued, dispatched or running task on the question's issue, from **Wake**, from an answer sent with Wake or from an ask back, MUST NOT be refused; once that agent has no queued, dispatched or running task left on the issue, one woken run of it SHALL start there with every answer and ask-back it has still not seen, and the timeline SHALL record who woke whom with how many answers. With nothing unseen then, no run SHALL start.

#### Scenario: Brian answers just before Pieter's run ends
- **GIVEN** Pieter asked Brian a question on WORK-997 and Pieter's run on WORK-997 is running
- **WHEN** Brian sends his answer "per line" with **Wake Pieter** checked
- **THEN** the answer is saved and the wake is not refused
- **AND** no run of Pieter is queued while that run runs
- **AND** once that run ends before it receives the answer, a new run of Pieter on WORK-997 starts with "per line" in its input
- **AND** the timeline reads "Pieter was woken by Brian with 1 answer"

#### Scenario: Pressing Wake while Pieter runs
- **GIVEN** Brian answered Pieter's question on WORK-997 and Pieter's run on WORK-997 is running
- **WHEN** Brian calls Wake for Pieter on the question
- **THEN** the wake is not refused and no run of Pieter is queued while that run runs
- **AND** once that run ends without marking the answer ✅, a new run of Pieter on WORK-997 starts with the answer

#### Scenario: The running run marked the answer ✅
- **GIVEN** Brian answered Pieter's question on WORK-997 with **Wake Pieter** checked while Pieter's run there was running
- **WHEN** that run receives the answer, marks it ✅ and ends
- **THEN** no new run of Pieter on WORK-997 starts

#### Scenario: Several wakes during one run
- **GIVEN** Pieter's run on WORK-997 is running
- **WHEN** Brian answers two of Pieter's questions there, each with **Wake Pieter** checked, and the run ends without marking either ✅
- **THEN** one new run of Pieter on WORK-997 starts, with both answers in its input

#### Scenario: A retry is queued when the run fails
- **GIVEN** Brian answered Pieter's question on WORK-997 with **Wake Pieter** checked while Pieter's run there was running
- **WHEN** that run fails and its retry is queued
- **THEN** no woken run starts while the retry is queued or runs
- **AND** once the retry ends with the answer still unseen, a woken run of Pieter on WORK-997 starts with the answer

#### Scenario: Pieter can no longer be woken when the run ends
- **GIVEN** Brian answered Pieter's question on WORK-997 with **Wake Pieter** checked while Pieter's run there was running
- **WHEN** Pieter is archived and that run then ends
- **THEN** no run of Pieter starts
- **AND** the answer stays unseen

#### Scenario: The wait survives a server restart
- **GIVEN** Brian answered Pieter's question on WORK-997 with **Wake Pieter** checked while Pieter's run there was running
- **WHEN** the server restarts and that run then ends without marking the answer ✅
- **THEN** a new run of Pieter on WORK-997 starts with the answer

## MODIFIED Requirements

### Requirement: Sending the last open question offers Wake
When Brian sends the last question of an ask still open for him, or the only question of a one-question ask, the card SHALL show **Wake {agent}** as a checkbox beside Send, checked by default. Sent checked, the answer SHALL wake the asking agent with every answer it has not seen, as the call to Wake does; while the agent still has a queued, dispatched or running task on the issue, its woken run starts once it has none left there.

#### Scenario: Brian answers the last question
- **GIVEN** Brian answered the first of Pieter's two questions on WORK-412 and Pieter's run has ended
- **WHEN** Brian sends the second one with **Wake Pieter** checked
- **THEN** a new run of Pieter on WORK-412 starts with both answers in its input

#### Scenario: Brian unchecks Wake
- **GIVEN** Pieter's one question on WORK-412 is open for Brian
- **WHEN** Brian unchecks **Wake Pieter** and sends his answer
- **THEN** no run is queued and the folded card offers **Wake Pieter**

#### Scenario: Brian answers the last question while Pieter runs
- **GIVEN** Pieter's run on WORK-412 is running and his one question there is open for Brian
- **WHEN** Brian sends his answer with **Wake Pieter** checked and that run then ends without marking the answer ✅
- **THEN** a new run of Pieter on WORK-412 starts with the answer in its input

### Requirement: Ask back wakes the asking agent
An ask back SHALL wake the asking agent with the reply; while it still has a queued, dispatched or running task on the question's issue, its woken run starts once it has none left there.

#### Scenario: Brian asks back
- **GIVEN** Pieter asked Brian a question on WORK-412 and Pieter's run has ended
- **WHEN** Brian asks back "Which customer is this for?"
- **THEN** a new run of Pieter on WORK-412 starts with the ask back in its input

#### Scenario: Brian asks back while Pieter runs
- **GIVEN** Pieter asked Brian a question on WORK-412 and Pieter's run on WORK-412 is running
- **WHEN** Brian asks back "Which customer is this for?" and that run then ends with the ask back still unseen
- **THEN** a new run of Pieter on WORK-412 starts with the ask back in its input
