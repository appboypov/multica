## ADDED Requirements

### Requirement: Every running run on the issue gets each item as a notice
Every running run on an issue SHALL receive each new comment, edited comment, answer, edited answer, ask-back and issue change on that issue as a notice at its next tool-batch boundary, after the work in progress finishes, without stopping that work, also when the item is not addressed to that run's agent.

#### Scenario: Brian replies in Skuddy's thread while Thomas runs
- **GIVEN** Skuddy is the assignee of WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian replies "use the toolbar copy button" in Skuddy's thread without mentioning Thomas
- **THEN** Thomas's run receives the reply as a notice, named as Brian's comment in Skuddy's thread
- **AND** no run is queued for Thomas

#### Scenario: A notice waits for the work in progress
- **GIVEN** Thomas's run on WORK-434 is running a build command
- **WHEN** Brian comments on WORK-434
- **THEN** the build command runs to its end
- **AND** Thomas's run receives the comment before its next model step

### Requirement: An answer reaches the running runs on the question's issues
When a person sends or changes an answer, or asks back, on a question linked to an issue, each running run on that issue SHALL receive the answer or ask-back as a notice, and no run SHALL be queued for it.

#### Scenario: Brian answers while Thomas runs
- **GIVEN** Thomas asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian sends the answer "use the toolbar copy button"
- **THEN** Thomas's run receives "use the toolbar copy button", named as Brian's answer to Thomas's question
- **AND** no run is queued for Thomas

#### Scenario: Brian answers Skuddy while Thomas runs
- **GIVEN** Skuddy asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian answers it
- **THEN** Thomas's run receives the answer, named as Brian's answer to Skuddy's question

#### Scenario: Brian changes his answer while Thomas runs
- **GIVEN** Thomas's running run on WORK-434 received Brian's answer "per line"
- **WHEN** Brian changes the answer to "per total"
- **THEN** Thomas's run receives "per total", named as Brian's changed answer

#### Scenario: Brian asks back while Thomas runs
- **GIVEN** Thomas asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian asks back "which toolbar?"
- **THEN** Thomas's run receives "which toolbar?", named as Brian's ask-back

### Requirement: A comment for a running agent joins its run
When a comment is posted on an issue for an agent that has a running run on that issue, through a mention, as the issue's assignee or in that agent's thread, that run SHALL receive the comment instead of a follow-up run being queued, also when another agent wrote the comment.

#### Scenario: Brian comments for Thomas while Thomas runs
- **GIVEN** Thomas is the assignee of WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian comments "keep the Markdown toggle" on WORK-434 without changing the recipient's choice
- **THEN** Thomas's run receives "keep the Markdown toggle", named as Brian's comment
- **AND** no follow-up run is queued for Thomas while the run runs

#### Scenario: Another agent comments while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Skuddy comments "@Thomas the modal needs it too" on WORK-434
- **THEN** Thomas's run receives the comment, named as Skuddy's comment

#### Scenario: A comment with files
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian comments for Thomas with the screenshot `toolbar.png` attached
- **THEN** Thomas's run receives the comment's text and the name and id of `toolbar.png`, with the command that downloads it

### Requirement: An edited comment reaches the running runs again
When a person or another agent changes the text of a comment on an issue, each running run on that issue SHALL receive the edited text as a new item, also when it received the comment before.

#### Scenario: Brian edits a comment Thomas's run received
- **GIVEN** Thomas's running run on WORK-434 received Brian's comment "keep the toggle"
- **WHEN** Brian edits it to "keep the toggle in the header"
- **THEN** Thomas's run receives "keep the toggle in the header", named as Brian's edited comment, with an item id of its own

#### Scenario: An edit that keeps the text
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian saves a comment without changing its text
- **THEN** Thomas's run receives nothing

### Requirement: The composer adds a comment to a running run by default
In the comment composer, a recipient whose run on the issue is running and takes input SHALL default to **Add to current run** in every thread, also for a comment with files, and **Start after this run**, **Stop and start over** and **Won't trigger** SHALL stay selectable. A recipient set to another choice than **Add to current run** SHALL NOT receive the comment in its running run.

#### Scenario: Default in a thread the run did not start
- **GIVEN** Thomas's run on WORK-434 is running and takes input
- **WHEN** Brian writes a top-level comment that mentions Thomas
- **THEN** Thomas's chip reads **Add to current run**
- **AND** its menu offers **Start after this run**, **Stop and start over** and **Won't trigger**

#### Scenario: Brian picks Start after this run
- **GIVEN** Thomas's chip offers **Start after this run**
- **WHEN** Brian picks it and sends the comment
- **THEN** Thomas's running run does not receive the comment
- **AND** the comment starts Thomas's follow-up run once the run ends

### Requirement: An issue change reaches every running run on the issue
When a person or another agent changes an issue's status, priority, assignee, title, description, start date, due date or labels, adds or removes a file on the issue, or adds or removes a sub-issue, each running run on that issue SHALL receive the change, naming who changed what from which value to which value.

#### Scenario: Brian raises the priority while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian sets WORK-434's priority from Medium to High
- **THEN** Thomas's run receives "Brian changed the priority from Medium to High"

#### Scenario: Brian edits the description while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian edits WORK-434's description
- **THEN** Thomas's run receives that Brian changed the description, with the new description

#### Scenario: Brian adds a label while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian adds the label "Design" to WORK-434
- **THEN** Thomas's run receives "Brian added the label Design"
- **AND** the issue's activity shows "Brian added the label Design"

#### Scenario: Brian attaches a file while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian attaches `flow.pdf` to WORK-434
- **THEN** Thomas's run receives that Brian attached `flow.pdf`, with its id and the command that downloads it
- **AND** the issue's activity shows "Brian attached flow.pdf"

#### Scenario: Brian adds a sub-issue while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian creates WORK-450 "Copy button tests" under WORK-434
- **THEN** Thomas's run receives "Brian added the sub-issue WORK-450 Copy button tests"
- **AND** WORK-434's activity shows "Brian added the sub-issue WORK-450"

#### Scenario: Brian moves a sub-issue away while Thomas runs
- **GIVEN** WORK-450 is a sub-issue of WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian moves WORK-450 under WORK-440
- **THEN** Thomas's run receives "Brian removed the sub-issue WORK-450 Copy button tests"

### Requirement: A run never receives its own agent's items
A running run SHALL NOT receive comments, edits, answers or issue changes made by its own agent, and an agent's comment during its run SHALL NOT queue that agent's follow-up run.

#### Scenario: Thomas comments while he runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Thomas comments "the toggle now sits in the header" on WORK-434
- **THEN** Thomas's run does not receive that comment
- **AND** no follow-up run is queued for Thomas

#### Scenario: Thomas changes the status while he runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Thomas sets WORK-434 to In Review
- **THEN** Thomas's run does not receive that change

### Requirement: An omp run takes input while it runs
A run on the omp runtime SHALL take the items above while it runs, as Claude Code, Codex and Grok runs do.

#### Scenario: Thomas runs on omp
- **GIVEN** Thomas runs on the Oh-My-Pi runtime and his run on WORK-434 is running
- **WHEN** Brian comments for Thomas on WORK-434
- **THEN** the comment shows as received by Thomas's run
- **AND** the run's transcript shows the comment as input after the step it was received in

### Requirement: A runtime without mid-run input keeps today's handling
When an agent's running run cannot take input, an item for it SHALL wait for the run to end and then get the handling it gets today.

#### Scenario: Brian comments for an agent on such a runtime
- **GIVEN** Anouk runs on a runtime that takes no input while it runs and her run on WORK-440 is running
- **WHEN** Brian comments for Anouk on WORK-440
- **THEN** Anouk's run does not receive the comment
- **AND** the comment starts Anouk's follow-up run once the run ends
