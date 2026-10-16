## ADDED Requirements

### Requirement: Check work in the to-do inbox
An agent SHALL open a check with an issue, project, work link, title, recipient and kind. Kinds SHALL be data model, data flow, design, prototype, feature, improvement, agent tool and release. The recipient SHALL answer pass or feedback. Feedback SHALL include a note. Answering SHALL wake the opening agent or deliver the answer to its active run. An automatic pass SHALL be returned to the opener without a separate run.

#### Scenario: Feature check
- **GIVEN** an agent has a task on an issue in a project
- **WHEN** it opens a feature check with a staging link
- **THEN** the recipient sees a Feature check to-do
- **AND** answering feedback saves the note and sends it to the opening agent

### Requirement: Keep an earned-trust record
The system SHALL store every answered check per project and kind, including automatic and sampled answers. Only human passes SHALL grow the clean streak. After the configured clean streak it SHALL open one proposal showing the record. Only the proposal recipient's yes SHALL make the kind automatic. A no SHALL require a fresh streak before another proposal.

#### Scenario: Agree to automatic checks
- **GIVEN** five human checks passed for a project's design kind
- **WHEN** the recipient says yes to the proposal
- **THEN** the project's design kind is automatic
- **AND** no other project or kind changes

### Requirement: Sample trusted work
Automatic kinds SHALL pass check opens automatically except each configured sample. Feedback on a sample SHALL return the kind to checked work and reset the streak. Releases SHALL always need a human check and SHALL never open trust proposals.

#### Scenario: Feedback on a sample
- **GIVEN** a project's feature kind is automatic
- **WHEN** its fifth automatic check is sampled and gets feedback
- **THEN** the feedback is recorded and the kind needs human checks again

#### Scenario: Release checks
- **GIVEN** a project's release checks have a clean streak
- **WHEN** an agent opens another release check
- **THEN** the recipient must answer it and no trust proposal opens

### Requirement: Manage check settings and view trust
Workspace settings SHALL default to five human passes and one sample in five automatic opens. Owners and admins SHALL be able to change them. The shared web and desktop inbox SHALL show checks, trust proposals, work links, answer actions and the selected project's trust record.

#### Scenario: Inbox answer
- **GIVEN** a member selects a check to-do
- **WHEN** they send feedback with a note
- **THEN** the action waits for the server and the handled row leaves the inbox
- **AND** a failed request keeps the row and shows an error
