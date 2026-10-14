## MODIFIED Requirements

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
