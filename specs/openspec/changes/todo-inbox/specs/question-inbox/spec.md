## REMOVED Requirements

### Requirement: Inbox items for answerers
**Reason**: A question asked of a member is now a to-do, which leaves once it is answered, as `todo-inbox` "A question or approval asked of a member is a to-do" says. An overdue question is an overdue to-do, not a second notice.
**Migration**: The open questions get their to-dos in a migration, and their unarchived `question_asked`, `question_reassigned` and `question_overdue` notices are archived.

## MODIFIED Requirements

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
