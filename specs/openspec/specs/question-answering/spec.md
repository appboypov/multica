# question-answering Specification

## Purpose

Lets answerers answer a question in its answer type, accept a suggestion, add a note, ask back and edit their answer.

## Answer types

| Answer type | Settings the asker gives | A valid answer |
|---|---|---|
| Text | short or long, placeholder | non-empty rich text, with mentions and attachments |
| Yes / No | optional "why" on No | Yes, or No with its why when asked for |
| Single choice | options, optional suggested one, optional Other | one option, or Other with its text |
| Multiple choice | options, min and max | between min and max options |
| Confirm facts | statements | each statement Correct, or Not correct with its correction |
| Ranking | items to order | every item in an order |
| Scale | range such as 1–5 or 0–10, end labels | one point in the range |
| Number | unit, min, max | a number from min to max |
| Date | date, date and time, or range | a value of that kind |
| Person | members, agents or both | one of the allowed people |
| File | accepted kinds | one or more files of those kinds |
| Approval | what is to be approved, with a link | Approve, Request changes with a note, or Decline |

## Requirements

### Requirement: Twelve answer types
An answerer SHALL answer a question in its answer type, and Multica MUST refuse to send an answer that does not meet the asker's settings for that type. The types, settings and controls are those of `designs/screens/issue-questions.md` "Answer types", summarised in the Answer types table above.

#### Scenario: Multiple choice below its minimum
- **GIVEN** a Multiple choice question with min 2 and max 3
- **WHEN** Brian picks one option
- **THEN** the answer cannot be sent and the card says at least 2 options are needed

#### Scenario: Request changes needs a note
- **GIVEN** an Approval question
- **WHEN** Brian picks Request changes without a note
- **THEN** the answer cannot be sent until Brian writes a note

#### Scenario: Single choice with Other
- **GIVEN** a Single choice question with Other allowed
- **WHEN** Brian picks Other and types "per invoice"
- **THEN** the sent answer is Other with the text "per invoice"

#### Scenario: Confirm facts with a correction
- **GIVEN** a Confirm facts question with three facts
- **WHEN** Brian marks two Correct and one Not correct without a correction
- **THEN** the answer cannot be sent until the Not correct fact has its correction

### Requirement: Accept the suggested answer
While nothing is picked or typed, a question with a suggested answer SHALL offer **Accept suggestion** as its primary action, which sends the suggested answer in one action; once the answerer picks or types, the primary action SHALL be **Send**.

#### Scenario: Brian accepts the suggestion
- **GIVEN** Pieter's question suggests "per line" and Brian has picked nothing
- **WHEN** Brian presses **Accept suggestion**
- **THEN** "per line" is sent as Brian's answer

#### Scenario: Picking turns the action into Send
- **GIVEN** Pieter's question suggests "per line"
- **WHEN** Brian picks "per total"
- **THEN** the primary action reads **Send**

### Requirement: Note and ask back
Every question card SHALL offer **Add a note** to send with the answer and **Ask back**, which replies in the question's thread and keeps the question open.

#### Scenario: Brian asks back
- **GIVEN** Pieter's question is open for Brian
- **WHEN** Brian uses **Ask back** with "Which customer is this for?"
- **THEN** the reply shows in the question's thread
- **AND** the question is still open and waiting for Brian

### Requirement: A sent answer reaches the asker at once
A sent answer SHALL be readable by the asker at once, and the card SHALL fold to one line: prompt → answer · who · when, with **Edit answer**.

#### Scenario: The card folds after sending
- **GIVEN** Brian answers Pieter's question with "per line"
- **WHEN** the answer is sent
- **THEN** the card shows one line with the prompt, "per line", Brian and the time, and **Edit answer**
- **AND** Pieter can read the answer through `multica question` without being woken

### Requirement: An answer stays editable until the asker closes the question
An answerer SHALL be able to edit a sent answer until the asker closes the question; each edit SHALL be readable by the asker at once and MUST NOT start a run.

#### Scenario: Brian edits his answer
- **GIVEN** Brian answered "per line" and the question is not closed
- **WHEN** Brian edits the answer to "per total"
- **THEN** Pieter reads "per total" through `multica question`
- **AND** no run is queued

#### Scenario: A closed question cannot be edited
- **GIVEN** Pieter closed the question Brian answered
- **WHEN** Brian opens its card
- **THEN** the card offers no **Edit answer**

### Requirement: Each answer of a set reaches the asker at once
Each answer sent to a question of a set SHALL be readable by the asker at once, without waiting for the other questions of the set, and the set's card SHALL show how many of its questions are answered.

#### Scenario: First answer of three
- **GIVEN** a question set of three open questions for Brian
- **WHEN** Brian sends the answer to the first question
- **THEN** Pieter reads that answer through `multica question` at once
- **AND** the card shows "1 of 3 answered"

#### Scenario: Leaving a set partly answered
- **GIVEN** a question set of three open questions for Brian
- **WHEN** Brian sends two answers and leaves the issue
- **THEN** the set stays open with "2 of 3 answered" and both sent answers stay sent
