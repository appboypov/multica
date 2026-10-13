## ADDED Requirements

### Requirement: A question is its own object
A question SHALL be an object of its own, never a comment, holding a prompt, an optional context line, one answer type with its settings, an optional suggested answer, one or more answerers, one or more links, an optional due date and the Blocks work flag.

#### Scenario: Asking adds a question, not a comment
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter asks Brian "Do invoices round per line or per total?" as a single-choice question
- **THEN** WORK-412 holds one question with that prompt, answer type Single choice and answerer Brian
- **AND** WORK-412 gains no comment

### Requirement: Only agents ask
Only an agent SHALL be able to ask a question; Multica MUST NOT offer members any way to create a question, and the issue's comment composer MUST NOT offer a Question toggle, a `/question` command or a question builder.

#### Scenario: A member cannot create a question
- **GIVEN** Brian is a member of the workspace
- **WHEN** Brian looks for a way to ask a question on an issue, in the API or in the app
- **THEN** none exists, and a member's request to create a question is refused

#### Scenario: The composer has no question builder
- **GIVEN** Brian opens WORK-412
- **WHEN** Brian types `/question` in the comment composer
- **THEN** no question builder opens and the composer shows no Question toggle

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
