# questions-page Specification

## Purpose
Lets a member see and filter every question that concerns them in one place, including the questions on work they are assigned to under On my work, and answer Yes / No questions there.

## Requirements

### Requirement: Questions sidebar item
The sidebar SHALL have an item **Questions**, under Inbox in the Personal group, showing the count of open questions for which the viewer is an answerer.

#### Scenario: Six open for Brian
- **GIVEN** Brian is the answerer of 6 open questions over four issues
- **WHEN** Brian looks at the sidebar
- **THEN** the Questions item shows 6

#### Scenario: Count drops as Brian answers
- **GIVEN** the Questions item shows 6
- **WHEN** Brian sends an answer to one of them
- **THEN** the Questions item shows 5

### Requirement: Questions page tabs
The Questions page SHALL have the tabs **For me** (open questions the viewer answers), **All open** (every open question in the workspace) and **Answered**, and an empty For me tab SHALL read "No questions waiting for you".

#### Scenario: For me
- **GIVEN** Brian is the answerer of 6 open questions and Anouk of 2 others
- **WHEN** Brian opens the Questions page on For me
- **THEN** it lists Brian's 6 questions and not Anouk's

#### Scenario: All open
- **GIVEN** the workspace has 8 open questions
- **WHEN** Brian opens All open
- **THEN** it lists all 8

#### Scenario: Nothing waiting
- **GIVEN** Brian answers no open question
- **WHEN** Brian opens For me
- **THEN** it reads "No questions waiting for you"

### Requirement: Grouping and filtering
The Questions page SHALL group its rows by link, by due date or by asker, and filter them on answer type, asker, answerer, link and blocking.

#### Scenario: Grouped by link
- **GIVEN** Brian's 6 open questions are on four issues
- **WHEN** Brian groups by link
- **THEN** the page shows four groups, one per issue

#### Scenario: Only blocking
- **GIVEN** 2 of Brian's 6 open questions block work
- **WHEN** Brian filters on blocking
- **THEN** the page lists those 2

### Requirement: Inline Yes / No
A Yes / No question SHALL be answerable from its row on the Questions page, by clicking Yes or No or pressing `Y` or `N` on the focused row.

#### Scenario: Brian answers a row
- **GIVEN** a Yes / No question for Brian is focused on the Questions page
- **WHEN** Brian presses `Y`
- **THEN** Yes is sent as Brian's answer and the row leaves For me

### Requirement: Rows show overdue and blocking
A row on the Questions page SHALL show an overdue badge while its question is open past its due date, and mark a question that blocks work.

#### Scenario: Overdue row
- **GIVEN** Brian's open question had due date 2026-10-14
- **WHEN** Brian opens For me on 2026-10-15
- **THEN** its row shows an overdue badge

### Requirement: On my work
The Questions page SHALL have a tab **On my work** listing the open questions whose links or parents have the viewer as member assignee, also when the viewer is not an answerer.

#### Scenario: Brian follows a question below his issue
- **GIVEN** Brian is the assignee of WORK-410 and Pieter asked Anouk a question on its sub-issue WORK-415
- **WHEN** Brian opens On my work
- **THEN** it lists the question, marked "via sub-issue WORK-415"
- **AND** the question is not under Brian's For me and offers Brian no answer controls
