# question-inbox Specification

## Purpose

Notifies answerers about their questions in the inbox and lets them answer simple ones there.

## Requirements

### Requirement: Inbox items for answerers
An answerer SHALL get an inbox item when a question is asked of them, when a question is reassigned to them, and when an open question of theirs passes its due date.

#### Scenario: Asked
- **GIVEN** Brian has no inbox item about Pieter's question
- **WHEN** Pieter asks Brian a question on WORK-412
- **THEN** Brian's inbox gets an item for that question that opens WORK-412 at its card

#### Scenario: Reassigned
- **GIVEN** Brian is the answerer of an open question
- **WHEN** Brian reassigns it to Anouk
- **THEN** Anouk's inbox gets an item for that question, with Brian's note

#### Scenario: Overdue
- **GIVEN** Brian's open question has due date 2026-10-14
- **WHEN** it is 2026-10-15 and the question is still open
- **THEN** Brian's inbox gets an overdue item for that question

### Requirement: Answer simple questions in the inbox
The inbox detail pane SHALL let the answerer answer a Yes / No, Single choice or Approval question in place, with the same rules as its card.

#### Scenario: Brian answers from the inbox
- **GIVEN** Brian's inbox item is for a Single choice question suggesting "per line"
- **WHEN** Brian accepts the suggestion in the inbox detail pane
- **THEN** "per line" is sent as Brian's answer and WORK-412's card folds to one line

#### Scenario: Other types open the issue
- **GIVEN** Brian's inbox item is for a Ranking question
- **WHEN** Brian opens it
- **THEN** the detail pane offers no inline answer and opens the question's card on its issue

### Requirement: No inbox item for an agent asker
Multica MUST NOT create a `question_answered` inbox item; the asking agent reads answers through `multica question`.

#### Scenario: Brian answers
- **GIVEN** Pieter asked Brian a question
- **WHEN** Brian sends his answer
- **THEN** no inbox item of type `question_answered` exists for anyone
