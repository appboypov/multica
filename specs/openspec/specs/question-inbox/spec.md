# question-inbox Specification

## Purpose

Notifies answerers about their questions in the inbox and lets them answer simple ones there.

## Requirements

### Requirement: Answer simple questions in the inbox
The inbox detail pane of a Question to-do SHALL let the answerer answer a Yes / No, Single choice or Approval question in place, with the same rules as its card.

#### Scenario: Brian answers from the inbox
- **GIVEN** Brian's Question to-do is for a Single choice question suggesting "per line"
- **WHEN** Brian accepts the suggestion in the inbox detail pane
- **THEN** "per line" is sent as Brian's answer, WORK-412's card folds to one line and the to-do leaves

#### Scenario: Other types open the issue
- **GIVEN** Brian's Question to-do is for a Ranking question
- **WHEN** Brian opens it
- **THEN** the detail pane offers no inline answer and opens the question's card on its issue

### Requirement: No inbox item for an agent asker
Multica MUST NOT create a `question_answered` inbox item; the asking agent reads answers through `multica question`.

#### Scenario: Brian answers
- **GIVEN** Pieter asked Brian a question
- **WHEN** Brian sends his answer
- **THEN** no inbox item of type `question_answered` exists for anyone
