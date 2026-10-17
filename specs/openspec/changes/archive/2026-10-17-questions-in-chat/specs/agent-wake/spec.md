## MODIFIED Requirements

### Requirement: An answer starts a run only with Wake checked
Sending, editing or cancelling MUST NOT queue or start any agent run unless it is sent with **Wake {agent}** checked; besides that, only a person pressing **Wake** or asking back SHALL start one.

#### Scenario: Brian answers and nothing starts
- **GIVEN** Pieter asked Brian two questions in one set on WORK-412, assigned to Pieter
- **WHEN** Brian sends his answer to the first one
- **THEN** no run is queued for Pieter or any other agent

#### Scenario: Brian cancels and nothing starts
- **GIVEN** Pieter asked Brian two questions in one set on WORK-412, assigned to Pieter
- **WHEN** Brian cancels the first one
- **THEN** no run is queued for Pieter or any other agent

### Requirement: Sending the last open question offers Wake
When Brian sends or cancels the last question of an ask still open for him, or the only question of a one-question ask, the card SHALL show **Wake {agent}** as a checkbox beside Send and beside Cancel, checked by default. Sent checked, the answer or cancel SHALL wake the asking agent with every answer and cancel it has not seen, as the call to Wake does; while the agent still has a queued, dispatched or running task on the issue, its woken run starts once it has none left there.

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

#### Scenario: Brian cancels the last question
- **GIVEN** Brian answered the first of Pieter's two questions on WORK-412 and Pieter's run has ended
- **WHEN** Brian cancels the second one with the note "already decided in the PRD" and **Wake Pieter** checked
- **THEN** a new run of Pieter on WORK-412 starts with the answer and the cancel, by Brian with that note, in its input

#### Scenario: Brian cancels with Wake unchecked
- **GIVEN** Pieter's one question on WORK-412 is open for Brian
- **WHEN** Brian unchecks **Wake Pieter** and cancels the question
- **THEN** no run is queued and the cancelled card offers **Wake Pieter**

### Requirement: Wake the asking agent from the answered card
Once a question has an answer or a cancel its asking agent has not seen, its answered or cancelled card SHALL offer **Wake** for that agent, named after it, such as **Wake Pieter**. An answer the agent marked ✅ in a run that received it counts as seen.

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

#### Scenario: Wake offered after a cancel
- **GIVEN** Brian cancelled Pieter's question on WORK-412 with Wake unchecked and Pieter has had no run since
- **WHEN** Brian looks at the cancelled card
- **THEN** it offers **Wake Pieter**

### Requirement: A woken run gets the answers it has not seen
A run started by **Wake** SHALL start with every answer and cancel given since the claim of that agent's latest started run on the question's links outside its retry ancestor chain, leaving out each answer that agent marked ✅ in a run that received it. Each cancel SHALL include who cancelled and its note, and the issue's timeline SHALL record who woke whom with how many answers.

#### Scenario: Brian wakes Pieter
- **GIVEN** Brian answered Pieter's question on WORK-412 with "per line"
- **WHEN** Brian presses **Wake Pieter**
- **THEN** a new run of Pieter on WORK-412 starts with the answer "per line" in its input
- **AND** the timeline reads "Pieter was woken by Brian with 1 answer"

#### Scenario: Brian wakes Pieter after a cancel
- **GIVEN** Brian cancelled Pieter's question on WORK-412 with the note "already decided in the PRD"
- **WHEN** Brian presses **Wake Pieter**
- **THEN** a new run of Pieter on WORK-412 starts with the cancel, by Brian with that note, in its input

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
- **GIVEN** a question-wake claim has no answers, cancels and ask-backs
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
