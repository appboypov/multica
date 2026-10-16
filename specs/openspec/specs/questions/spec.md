# questions Specification

## Purpose

Lets agents ask people questions as objects of their own, with member answerers, links, a due date, Blocks work and a lifecycle.

## Requirements

### Requirement: A question is its own object
A question SHALL be an object of its own, never a comment, holding a prompt, an optional context line, one answer type with its settings, an optional suggested answer, one or more answerers, one or more links, an optional due date and the Blocks work flag.

#### Scenario: Asking adds a question, not a comment
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter asks Brian "Do invoices round per line or per total?" as a single-choice question
- **THEN** WORK-412 holds one question with that prompt, answer type Single choice and answerer Brian
- **AND** WORK-412 gains no comment

### Requirement: Only members answer
Every answerer of a question MUST be a member; an agent MUST NOT be an answerer.

#### Scenario: An agent named as answerer is refused
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter asks a question with the agent Joeri as its answerer
- **THEN** the ask is refused with an error that names Joeri as an agent
- **AND** no question is posted

### Requirement: A question asked during a run links to where it was asked
A question asked during a run SHALL link to that run and to the run's issue, and its first link SHALL be where it was asked.

#### Scenario: Links of a question asked in run #3
- **GIVEN** Pieter is in run #3 on WORK-412
- **WHEN** Pieter asks a question
- **THEN** the question links to WORK-412 and to run #3
- **AND** WORK-412 is its first link

### Requirement: Question lifecycle
A question SHALL be open until answered, answered once its answer is sent, and closed only when the asker closes it; the asker SHALL be able to withdraw an open or answered question, and a withdrawn question MUST leave every open list.

#### Scenario: The asker closes an answered question
- **GIVEN** Brian answered Pieter's question
- **WHEN** Pieter closes it
- **THEN** the question is closed and its answer can no longer be edited

#### Scenario: The asker withdraws a question
- **GIVEN** Pieter's question on WORK-412 is open
- **WHEN** Pieter withdraws it
- **THEN** it no longer counts as open on WORK-412 and no longer shows as waiting for Brian

### Requirement: Due date and overdue
The asker SHALL be able to give a question a due date; an open question past its due date SHALL show an overdue badge on its card.

#### Scenario: An open question passes its due date
- **GIVEN** Pieter's question has due date 2026-10-14 and is open
- **WHEN** it is 2026-10-15
- **THEN** its card shows an overdue badge

#### Scenario: An answered question shows no overdue badge
- **GIVEN** Pieter's question has due date 2026-10-14
- **WHEN** Brian answered it on 2026-10-13 and it is 2026-10-15
- **THEN** its card shows no overdue badge

### Requirement: Blocks work
The asker SHALL be able to set Blocks work on a question, marking the linked work as waiting on its answer.

#### Scenario: A blocking question
- **GIVEN** Pieter asks a question with Blocks work on, linked to WORK-412
- **WHEN** Brian opens WORK-412
- **THEN** the question is marked as blocking work on WORK-412

### Requirement: Reassign with a note
An answerer SHALL be able to reassign an open question to another member with a note, and the question SHALL keep the reassignment in its history.

#### Scenario: Brian reassigns to Anouk
- **GIVEN** Brian is the answerer of an open question
- **WHEN** Brian reassigns it to Anouk with the note "You set the invoice rules"
- **THEN** Anouk is its answerer and Brian no longer is
- **AND** the card shows that Brian reassigned it to Anouk, with the note

### Requirement: Asking starts nothing
Posting a question MUST NOT queue or start any agent run.

#### Scenario: No run after an ask
- **GIVEN** Pieter asks Brian a question on WORK-412, whose assignee is the agent Pieter
- **WHEN** the question is posted
- **THEN** no run is queued for any agent

### Requirement: Question sets
One ask SHALL hold one question or a question set of several questions, and each question in a set SHALL be answerable and sendable without answering the others.

#### Scenario: Three questions in one ask
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter asks three questions in one ask
- **THEN** WORK-412 holds one question set of three questions

#### Scenario: Answering one question of a set
- **GIVEN** a question set of three open questions for Brian
- **WHEN** Brian sends an answer to the second question only
- **THEN** the second question is answered and the first and third are still open

### Requirement: Open counts count questions
Every count of open questions SHALL count questions, not asks.

#### Scenario: A set of three
- **GIVEN** WORK-412 has one question set of three open questions and no other question
- **WHEN** Brian reads WORK-412's open question count
- **THEN** it is 3

### Requirement: Asking a squad
Asking a squad SHALL make each of its human members an answerer and MUST skip its agents.

#### Scenario: Squad of three members and two agents
- **GIVEN** squad Finance has the members Brian, Anouk and Roos and the agents Joeri and Thomas
- **WHEN** Pieter asks a question of squad Finance
- **THEN** its answerers are Brian, Anouk and Roos

### Requirement: Answer policy
A question with more than one answerer SHALL have the answer policy **Any one** or **Each**, picked by the asker. Under Any one, the first sent answer SHALL make the question answered and take it off the other answerers' open lists, and only the asker closes it. Under Each, the question SHALL be partly answered until every answerer has sent an answer, then answered.

#### Scenario: Any one
- **GIVEN** a question asked of Brian, Anouk and Roos under Any one
- **WHEN** Brian sends his answer
- **THEN** the question is answered and is no longer open for Anouk and Roos
- **AND** it is not closed, so Brian can still edit his answer

#### Scenario: Each
- **GIVEN** a question asked of Brian, Anouk and Roos under Each
- **WHEN** Brian and Anouk have sent their answers
- **THEN** the question is partly answered, its card shows "2 of 3", and it is still open for Roos

#### Scenario: Each, completed
- **GIVEN** a question under Each with two of three answers sent
- **WHEN** Roos sends her answer
- **THEN** the question is answered

### Requirement: Blocks work is on by default during a run
A question an agent asks during a run SHALL have Blocks work on unless the ask turns it off.

#### Scenario: Asked during a run
- **GIVEN** Pieter is in a run on WORK-412
- **WHEN** Pieter asks a question without saying anything about Blocks work
- **THEN** the question blocks work on WORK-412

#### Scenario: Turned off
- **GIVEN** Pieter is in a run on WORK-412
- **WHEN** Pieter asks a question with Blocks work off
- **THEN** the question does not block work on WORK-412

### Requirement: Link a question to any workspace object
A question SHALL be able to link to an issue, a sub-issue, a project, a comment, an agent run or another workspace object, and to more than one of them; the asking agent SHALL be able to add links after the first.

#### Scenario: Asked on a comment
- **GIVEN** Pieter runs on WORK-415
- **WHEN** Pieter asks a question linked to a comment on WORK-415
- **THEN** the question links to that comment

#### Scenario: Adding a link later
- **GIVEN** Pieter's open question links to WORK-415
- **WHEN** Pieter adds a link to project Agent runtime through `multica question`
- **THEN** the question links to WORK-415 and to project Agent runtime, with WORK-415 still its first link

### Requirement: Parents of a link
Every object above a question's link SHALL be a parent of the question: a comment's issue, an issue's parent issues up the chain, and their project.

#### Scenario: Sub-issue in a project
- **GIVEN** WORK-415 is a sub-issue of WORK-410, which is in project Agent runtime
- **WHEN** Pieter asks a question linked to WORK-415
- **THEN** WORK-410 and project Agent runtime are its parents

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
