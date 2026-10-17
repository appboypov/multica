# check-todos Specification

## Purpose
Agents ask the recipient to check their work in the to-do inbox and each project keeps a record of the answers per check kind.

## Requirements

### Requirement: Check work in the to-do inbox
An agent SHALL open a check with an issue, project, work link, title, recipient and kind. Kinds SHALL be data model, data flow, design, prototype, feature, improvement, agent tool and release. Only the recipient SHALL answer pass or feedback. Feedback SHALL include a note. The answer SHALL reach the opening agent through an active run input or queued run. Check to-dos SHALL not allow Mark done.

#### Scenario: Feature feedback
- **GIVEN** an agent has a task on an issue in a project
- **WHEN** Brian answers a feature check with feedback
- **THEN** the answer and note are stored and sent to that agent
- **AND** the message asks it to fix the piece, change the instruction and post an instruction note

### Requirement: Keep the project record and history
The system SHALL store every answered check per project and kind, including automatic passes and sampled answers. The app SHALL show counts and history with work links, answers and notes. Counts and answers SHALL never change the checking mode.

#### Scenario: Passed checks keep the mode
- **GIVEN** a project and kind checks every piece
- **WHEN** Brian passes ten checks
- **THEN** it still checks every piece and all ten answers appear in its history

### Requirement: Brian chooses every piece or samples
Only the record's recipient SHALL switch a project and kind between checking every piece and samples. Samples SHALL send every fifth piece by default, using a deterministic counter. Feedback on a sample SHALL keep its mode. A sampled check SHALL offer a one-click action to check every piece. Release SHALL always check every piece.

#### Scenario: Brian chooses samples and receives feedback
- **GIVEN** Brian switches a project's feature kind to samples
- **WHEN** its fifth piece is sampled and gets feedback
- **THEN** the feedback is recorded and the kind stays on samples
- **AND** only Brian's explicit action switches it to every piece

#### Scenario: Release checks
- **GIVEN** Brian opens the release record
- **WHEN** he asks to use samples
- **THEN** the request is refused and every release still needs his answer

### Requirement: Instruction changes have an undo note
The opening agent SHALL post an instruction note through the API or CLI for an answered feedback check. It SHALL name the changed file, summary and commit link. Only that agent in a task on the same issue SHALL post the note and its undo result. Brian SHALL choose Keep or Undo. Undo SHALL wake that agent with a request to revert the change and report back on the same note. Keep SHALL close it. Repeated requests SHALL not create duplicate notes or wakes.

#### Scenario: Undo an instruction change
- **GIVEN** the opening agent has posted its instruction change note
- **WHEN** Brian chooses Undo
- **THEN** the note stays open and the same agent receives the revert request and report command
- **AND** its reported commit and result appear on that note until Brian chooses Keep

### Requirement: Manage sample settings
Workspace settings SHALL default to `sample_every` 5. Only workspace owners and admins SHALL change the value, within 1 to 100. The UI SHALL wait for server actions and show request failures without claiming success.

#### Scenario: Failed mode change
- **GIVEN** Brian views a project's record
- **WHEN** a mode change request fails
- **THEN** its mode stays unchanged in the app and an error is shown
