# agent-wake Specification

## Purpose

Lets a person start an agent with the answers it has not seen: by pressing **Wake**, by sending the last open question of an ask with **Wake {agent}** checked, or by asking back.

## Requirements

### Requirement: An answer starts a run only with Wake checked
Sending or editing an answer MUST NOT queue or start any agent run unless it is sent with **Wake {agent}** checked; besides that, only a person pressing **Wake** or asking back SHALL start one.

#### Scenario: Brian answers and nothing starts
- **GIVEN** Pieter asked Brian two questions in one set on WORK-412, assigned to Pieter
- **WHEN** Brian sends his answer to the first one
- **THEN** no run is queued for Pieter or any other agent

### Requirement: Sending the last open question offers Wake
When Brian sends the last question of an ask still open for him, or the only question of a one-question ask, the card SHALL show **Wake {agent}** as a checkbox beside Send, checked by default. Sent checked, the answer wakes the asking agent with every answer it has not seen; when the agent cannot be woken now, such as while it still runs on the issue, the answer is saved and rides along in its next woken run.

#### Scenario: Brian answers the last question
- **GIVEN** Brian answered the first of Pieter's two questions on WORK-412 and Pieter's run has ended
- **WHEN** Brian sends the second one with **Wake Pieter** checked
- **THEN** a new run of Pieter on WORK-412 starts with both answers in its input

#### Scenario: Brian unchecks Wake
- **GIVEN** Pieter's one question on WORK-412 is open for Brian
- **WHEN** Brian unchecks **Wake Pieter** and sends his answer
- **THEN** no run is queued and the folded card offers **Wake Pieter**

### Requirement: Ask back wakes the asking agent
An ask back SHALL wake the asking agent with the reply when it has no queued, dispatched or running task on the question's issue; otherwise the reply stays unseen and rides along in its next woken run.

#### Scenario: Brian asks back
- **GIVEN** Pieter asked Brian a question on WORK-412 and Pieter's run has ended
- **WHEN** Brian asks back "Which customer is this for?"
- **THEN** a new run of Pieter on WORK-412 starts with the ask back in its input

### Requirement: Wake the asking agent from the answered card
Once a question has an answer its asking agent has not seen, its answered card SHALL offer **Wake** for that agent, named after it, such as **Wake Pieter**. An answer the agent marked ✅ in a run that received it counts as seen.

#### Scenario: Wake offered after the answer
- **GIVEN** Brian answered Pieter's question on WORK-412 and Pieter has had no run since
- **WHEN** Brian looks at the folded card
- **THEN** it offers **Wake Pieter**

#### Scenario: No Wake once the agent saw the answer
- **GIVEN** Pieter was woken after Brian's answer and no new answer has been given since
- **WHEN** Brian looks at the folded card
- **THEN** it offers no **Wake Pieter**

#### Scenario: No Wake once the running agent marked the answer ✅
- **GIVEN** Pieter's run on WORK-412 was running when Brian answered, received the answer and marked it ✅
- **WHEN** the run ends and Brian looks at the folded card
- **THEN** it offers no **Wake Pieter**

#### Scenario: Wake after a run that left the answer without ✅
- **GIVEN** Pieter's run on WORK-412 received Brian's answer and marked it 👀 only
- **WHEN** the run ends and Brian looks at the folded card
- **THEN** it offers **Wake Pieter**

### Requirement: A woken run gets the answers it has not seen
A run started by **Wake** SHALL start with every answer given since the claim of that agent's latest started run on the question's links outside its retry ancestor chain, leaving out each answer that agent marked ✅ in a run that received it, and the issue's timeline SHALL record who woke whom with how many answers.

#### Scenario: Brian wakes Pieter
- **GIVEN** Brian answered Pieter's question on WORK-412 with "per line"
- **WHEN** Brian presses **Wake Pieter**
- **THEN** a new run of Pieter on WORK-412 starts with the answer "per line" in its input
- **AND** the timeline reads "Pieter was woken by Brian with 1 answer"

#### Scenario: An answer changes while the woken run is dispatched
- **GIVEN** Pieter's woken run has been claimed with Brian's answer "per line" but has not started
- **WHEN** Brian changes the answer to "per total" and that run then starts and ends
- **THEN** the changed answer remains unseen and **Wake Pieter** is available
- **AND** the next woken run receives "per total"

#### Scenario: A woken run retries
- **GIVEN** Pieter's woken run received Brian's answer and ask-back, then failed
- **WHEN** the run retries, including a retry of a retry
- **THEN** every attempt receives the answer and ask-back without its retry ancestors advancing the unseen cutoff

#### Scenario: No answers or ask-backs are available at claim
- **GIVEN** a question-wake claim has no answers and no ask-backs
- **WHEN** the daemon builds its prompt
- **THEN** the prompt has no `[ANSWERS]` block

#### Scenario: A claim fails before the agent starts
- **GIVEN** Brian answered Pieter and the woken run was claimed
- **WHEN** that claim fails before the agent starts and no retry is queued
- **THEN** the answer remains unseen and **Wake Pieter** is available

#### Scenario: Wake leaves out the answer marked ✅
- **GIVEN** Pieter's running run received and marked ✅ Brian's first answer, and Brian answered a second question of the set after that run ended
- **WHEN** Brian presses **Wake Pieter**
- **THEN** the new run starts with only the second answer
- **AND** the timeline reads "Pieter was woken by Brian with 1 answer"

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

### Requirement: Wake within Answer all
In Answer all, the bar under the card in One by one and the end of each link's cards in One page SHALL offer **Wake** for each agent that has answers on that link it has not seen, naming how many, such as "Pieter has 1 answer from you on WORK-412".

#### Scenario: One by one
- **GIVEN** Brian just answered Pieter's question on WORK-412 in One by one
- **WHEN** the answer is sent
- **THEN** the bar under the card reads "Pieter has 1 answer from you on WORK-412" and offers **Wake Pieter**
- **AND** no run is queued for Pieter

#### Scenario: One page
- **GIVEN** Brian answered both of Pieter's questions on WORK-398 on One page
- **WHEN** Brian reaches the end of WORK-398's cards
- **THEN** **Wake Pieter** is offered there
- **AND** pressing it starts Pieter's run with both answers

### Requirement: Wake the agent assignees of links and parents
Wherever **Wake** is offered, it SHALL be offered for the asking agent and for each agent assignee of the question's links and parents that has answers on those links it has not seen; a woken agent assignee SHALL get every answer given since its last run on those links.

#### Scenario: The parent's agent
- **GIVEN** Pieter asked Anouk a question on WORK-415, whose parent WORK-410 is assigned to the agent Joeri, and Anouk answered
- **WHEN** Brian looks at the answered card
- **THEN** it offers **Wake Pieter** and **Wake Joeri**

#### Scenario: Waking the parent's agent
- **GIVEN** the answered card offers **Wake Joeri**
- **WHEN** Brian presses it
- **THEN** a run of Joeri starts with Anouk's answer
- **AND** no run of Pieter is queued

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
