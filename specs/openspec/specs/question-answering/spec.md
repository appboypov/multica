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
Every question card SHALL offer **Add a note** to send with the answer and **Ask back**, which replies in the question's thread, keeps the question open and wakes the asking agent.

#### Scenario: Brian asks back
- **GIVEN** Pieter's question is open for Brian
- **WHEN** Brian uses **Ask back** with "Which customer is this for?"
- **THEN** the reply shows in the question's thread
- **AND** the question is still open and waiting for Brian
- **AND** Pieter is woken with the reply

### Requirement: Decline to answer
Every open question card SHALL offer **Decline to answer**, which sends a declined answer, with the note as its optional reason; it counts as the answerer's answer and reads "Declined to answer".

#### Scenario: Brian declines
- **GIVEN** Pieter's question is open for Brian
- **WHEN** Brian uses **Decline to answer**
- **THEN** the card folds to the prompt → "Declined to answer"
- **AND** Pieter reads "Declined to answer" as Brian's answer

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

### Requirement: Cancel a question
Every card that offers a current answerer answer controls SHALL also offer **Cancel question**, with an optional note as its reason; a cancel SHALL end the question for every answerer, take it off every open list and count, and MUST leave the answers already sent readable.

#### Scenario: Brian cancels a question asked of three
- **GIVEN** Pieter asked Brian, Anouk and Roos one question on WORK-412 under Each and Anouk already answered
- **WHEN** Brian cancels it with the note "already decided in the PRD"
- **THEN** the card reads "Cancelled by Brian" with the note and offers no answer controls to anyone
- **AND** the question leaves the For me lists and sidebar counts of Brian and Roos
- **AND** Anouk's answer is still shown on the card

#### Scenario: Cancelling without a note
- **GIVEN** Pieter's question on WORK-412 is open for Brian
- **WHEN** Brian cancels it without a note
- **THEN** the card reads "Cancelled by Brian"

#### Scenario: Pieter reads the cancel
- **GIVEN** Brian cancelled Pieter's question with the note "already decided in the PRD"
- **WHEN** Pieter reads the question through `multica question`
- **THEN** the output shows the question cancelled, by Brian, with that note and when

#### Scenario: A member who is not an answerer
- **GIVEN** Pieter asked Brian a question on WORK-412
- **WHEN** Anouk, who is not an answerer, opens WORK-412
- **THEN** the card offers her no **Cancel question**

#### Scenario: A cancelled question stays ended
- **GIVEN** Brian cancelled Pieter's question
- **WHEN** Brian opens its card
- **THEN** it offers no answer controls and no **Edit answer**
- **AND** Pieter can neither close nor withdraw it
