# agent-wake Specification

## Purpose

Lets a person start an agent with the answers it has not seen; asking and answering questions never start a run.

## Requirements

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
A run started by **Wake** SHALL start with every answer given since the claim of that agent's latest started run on the question's links outside its retry ancestor chain, and the issue's timeline SHALL record who woke whom with how many answers.

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
