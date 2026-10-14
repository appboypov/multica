## ADDED Requirements

### Requirement: Answer all
**Answer all** on the Questions page SHALL open the viewer's open questions in one flow, in the view **One by one** or **One page**, switchable in its header; each answer SHALL be sent on its own.

#### Scenario: Starting Answer all
- **GIVEN** Brian has 6 open questions
- **WHEN** Brian presses **Answer all**
- **THEN** a flow of his 6 open questions opens, with One by one and One page in its header

### Requirement: One by one
In **One by one**, Answer all SHALL show one question at a time with its link's context beside it and the progress, such as "2 of 6"; `J` and `K` SHALL move between questions, `S` SHALL skip and `Esc` SHALL leave.

#### Scenario: Moving through
- **GIVEN** Brian is on question 2 of 6 in One by one
- **WHEN** Brian sends its answer
- **THEN** question 3 shows with its issue's context beside it and the progress "3 of 6"

#### Scenario: Skipping
- **GIVEN** Brian is on question 3 of 6 in One by one
- **WHEN** Brian presses `S`
- **THEN** question 4 shows and question 3 stays open

### Requirement: One page
In **One page**, Answer all SHALL show every open question of the viewer on one page, grouped by link, each answerable where it stands, with a question set's questions all shown on its card; `J` and `K` SHALL move the focus between questions and `⌘↵` SHALL send the focused one.

#### Scenario: Six questions on one page
- **GIVEN** Brian has 6 open questions on four issues, three of them in one set
- **WHEN** Brian switches to One page
- **THEN** the page shows four groups with all 6 questions answerable, the set's three on its card

#### Scenario: Sending from the keyboard
- **GIVEN** a Text question is focused on One page and Brian typed an answer
- **WHEN** Brian presses `⌘↵`
- **THEN** that answer is sent and its card folds to one line

### Requirement: Answer all remembers the view
Answer all SHALL open in the view the viewer used last.

#### Scenario: Reopening
- **GIVEN** Brian last used One page
- **WHEN** Brian presses **Answer all** the next day
- **THEN** it opens in One page

### Requirement: Leaving halfway
Leaving Answer all SHALL keep every sent answer and leave every unanswered question open.

#### Scenario: Brian leaves after three
- **GIVEN** Brian sent 3 of his 6 answers in Answer all
- **WHEN** Brian leaves
- **THEN** the 3 answers are sent and the other 3 questions are open under For me
