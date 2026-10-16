## REMOVED Requirements

### Requirement: Only agents ask
**Reason**: Sessions outside a run, such as a Herdr omp session with Skuddy's login, ask Brian questions now that his inbox moves to Multica.
**Migration**: "Agents ask in a run and members ask outside the app" replaces it; the app still offers no way to ask.

## ADDED Requirements

### Requirement: Agents ask in a run and members ask outside the app
An agent SHALL ask a question from a run with its task token, and a member SHALL ask a question with a member login through the API and `multica question ask` outside a run; machine credentials MUST NOT ask, and the app MUST NOT offer members any way to create a question: the issue's comment composer MUST NOT offer a Question toggle, a `/question` command or a question builder.

#### Scenario: A member asks through the API
- **GIVEN** Skuddy is a member of the workspace and his omp session holds his login
- **WHEN** the session asks Brian "Pass the plan for WORK-442?" on WORK-442 through the API
- **THEN** WORK-442 holds the question with Skuddy as its asker and Brian as its answerer

#### Scenario: A machine credential cannot ask
- **GIVEN** a cloud runtime authenticates with its cloud personal access token
- **WHEN** it asks a question on WORK-442
- **THEN** the ask is refused with status 403
- **AND** no question is posted

#### Scenario: The app offers no way to ask
- **GIVEN** Brian opens WORK-412 in the app
- **WHEN** Brian looks for a way to ask a question
- **THEN** none exists

#### Scenario: The composer has no question builder
- **GIVEN** Brian opens WORK-412
- **WHEN** Brian types `/question` in the comment composer
- **THEN** no question builder opens and the composer shows no Question toggle

### Requirement: A question asked outside a run is asked on an issue
A question a member asks SHALL have an issue of the workspace as its first link and no run link, and an ask without an issue as its first link MUST be refused.

#### Scenario: Links of Skuddy's question
- **GIVEN** Skuddy's omp session holds his login and has no run
- **WHEN** it asks Brian a question on WORK-442 that also links WORK-435
- **THEN** the question's first link is WORK-442, its second link is WORK-435 and it links no run

#### Scenario: No issue first
- **GIVEN** Skuddy's omp session holds his login and has no run
- **WHEN** it asks a question whose first link is the project Multica fork
- **THEN** the ask is refused with status 422 and an error that says a question asked outside a run is asked on an issue
- **AND** no question is posted

#### Scenario: No links
- **GIVEN** Skuddy's omp session holds his login and has no run
- **WHEN** it asks a question without links
- **THEN** the ask is refused with status 422
- **AND** no question is posted

### Requirement: Agent questions does not limit a member's ask
A member's ask SHALL be accepted whatever the issue's **Agent questions** setting and the workspace's Questions default say, since those settings govern the runs of agents.

#### Scenario: Questions off on the issue
- **GIVEN** WORK-442 has Agent questions set to Off
- **WHEN** Skuddy's omp session asks Brian a question on WORK-442
- **THEN** WORK-442 holds the question with Skuddy as its asker

### Requirement: Every question view names its asker
The question card, the question set card, Answer all, the Questions page row and its asker filter and grouping SHALL name and show the avatar of the question's asker, an agent or a member; a question a member asked SHALL read "asked · {time}" where an agent's question reads its run, and its answerer progress line MUST NOT say that the asker closes the question.

#### Scenario: Skuddy's card
- **GIVEN** Skuddy's omp session asked Brian a question on WORK-442 a minute ago
- **WHEN** Brian opens WORK-442
- **THEN** the card shows Skuddy's avatar and name with "asked · 1m ago" and no run number

#### Scenario: Questions page by asker
- **GIVEN** Pieter asked Brian a question in a run and Skuddy asked Brian a question outside a run
- **WHEN** Brian groups the Questions page by asker and filters it on Skuddy
- **THEN** there is a group for Pieter and a group for Skuddy
- **AND** the filter leaves only Skuddy's question

#### Scenario: Pieter's card is unchanged
- **GIVEN** Pieter asked Brian a question in run #3 on WORK-412 a minute ago
- **WHEN** Brian opens WORK-412
- **THEN** the card shows Pieter's avatar and name with "asked during run #3 · 1m ago"

#### Scenario: Progress line of Skuddy's question
- **GIVEN** Skuddy asked Brian and Anouk a question on WORK-442 with answer policy each, and Brian answered
- **WHEN** Brian looks at the card
- **THEN** the progress line reads "Skuddy sees each answer as it arrives." and says nothing about closing the question

### Requirement: The asker's login hears its questions live
Every answer, answer edit, ask back and decline of a question a member asked SHALL reach that member's live connection (`/ws`) as `question:updated` naming the question, its ask and the reason.

#### Scenario: Brian answers
- **GIVEN** Skuddy's omp session asked Brian a question on WORK-442 and holds a live connection with Skuddy's login
- **WHEN** Brian answers it
- **THEN** the connection receives `question:updated` with the question's id, its ask id and reason `answered`

#### Scenario: Brian asks back
- **GIVEN** Skuddy's omp session asked Brian a question on WORK-442 and holds a live connection with Skuddy's login
- **WHEN** Brian asks back "Which plan do you mean?"
- **THEN** the connection receives `question:updated` with the question's id and reason `ask_back`
- **AND** reading the question shows the ask back in its events

#### Scenario: Brian declines
- **GIVEN** Skuddy's omp session asked Brian a question on WORK-442 and holds a live connection with Skuddy's login
- **WHEN** Brian uses **Decline to answer**
- **THEN** the connection receives `question:updated` with the question's id and reason `answered`
- **AND** reading the question shows Brian's answer as declined

#### Scenario: Brian changes his answer
- **GIVEN** Brian answered Skuddy's question on WORK-442
- **WHEN** Brian changes the answer
- **THEN** Skuddy's live connection receives `question:updated` with reason `answer_edited`
