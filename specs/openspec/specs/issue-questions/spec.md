# issue-questions Specification

## Purpose

Lets everyone on an issue see its questions as cards, the run that waits on them, and the open count in the properties.

## Requirements

### Requirement: Question card in the issue timeline
On an issue, each question linked to it SHALL show as a card in the activity timeline, at the place it was asked, and the card SHALL be outlined with a "Question for you" overline when the viewer is one of its answerers.

#### Scenario: Brian sees a question for him
- **GIVEN** Pieter asked Brian a question on WORK-412
- **WHEN** Brian opens WORK-412
- **THEN** the timeline shows the question's card at the moment it was asked, outlined with the overline "Question for you"

#### Scenario: Another member sees the card without the overline
- **GIVEN** Pieter asked Brian a question on WORK-412
- **WHEN** Anouk, who is not an answerer, opens WORK-412
- **THEN** the card shows without the "Question for you" overline and without answer controls

### Requirement: Run row of a run that asked
The run row of an agent run that asked a question SHALL read "ended run #N with a question" and carry a "Waiting for answer" badge until the question is answered.

#### Scenario: Run #3 waits
- **GIVEN** Pieter asked a question during run #3 on WORK-412 and the run ended
- **WHEN** Brian opens WORK-412
- **THEN** the run row reads "ended run #3 with a question" with a "Waiting for answer" badge

#### Scenario: The badge leaves once answered
- **GIVEN** run #3 on WORK-412 shows "Waiting for answer"
- **WHEN** Brian sends his answer
- **THEN** the run row no longer shows the "Waiting for answer" badge

### Requirement: Questions property on an issue
An issue's properties SHALL show a **Questions** row, after Priority, with the count of its open questions and, when one of them blocks work, "blocks work"; selecting it SHALL scroll to the first open question card.

#### Scenario: One blocking question
- **GIVEN** WORK-412 has one open question with Blocks work on
- **WHEN** Brian opens WORK-412
- **THEN** its Questions row reads "1 open · blocks work"

#### Scenario: Jump to the card
- **GIVEN** WORK-412's Questions row reads "1 open · blocks work"
- **WHEN** Brian selects the row
- **THEN** the timeline scrolls to the open question card

### Requirement: Question set card
On an issue, a question set SHALL show as one card holding each of its questions, each with its own answer controls, and a footer for the set as a whole.

#### Scenario: A set of three on WORK-412
- **GIVEN** Pieter asked a question set of three questions on WORK-412 with Blocks work on
- **WHEN** Brian opens WORK-412
- **THEN** the timeline shows one card with the three questions, each answerable on its own
- **AND** WORK-412's Questions row reads "3 open · blocks work"
