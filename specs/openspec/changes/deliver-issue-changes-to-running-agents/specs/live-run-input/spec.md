## ADDED Requirements

### Requirement: An answer reaches the asking agent's running run
When a person sends or changes an answer, or asks back, on a question whose asking agent has a running run on an issue the question is linked to, that run SHALL receive the answer or ask-back at its next step, and no run SHALL be queued for it.

#### Scenario: Brian answers while Thomas runs
- **GIVEN** Thomas asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian sends the answer "use the toolbar copy button"
- **THEN** Thomas's run receives "use the toolbar copy button" at its next step, named as Brian's answer to that question
- **AND** no run is queued for Thomas

#### Scenario: Brian changes his answer while Thomas runs
- **GIVEN** Thomas's running run on WORK-434 received Brian's answer "per line"
- **WHEN** Brian changes the answer to "per total"
- **THEN** Thomas's run receives "per total" at its next step, named as Brian's changed answer

#### Scenario: Brian asks back while Thomas runs
- **GIVEN** Thomas asked Brian a question on WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian asks back "which toolbar?"
- **THEN** Thomas's run receives "which toolbar?" at its next step, named as Brian's ask-back

### Requirement: A comment reaches the running run of the agent it is for
When a comment is posted on an issue for an agent that has a running run on that issue, through a mention, as the issue's assignee or in that agent's thread, that run SHALL receive the comment at its next step instead of a follow-up run being queued, also when another agent wrote the comment.

#### Scenario: Brian comments while Thomas runs
- **GIVEN** Thomas is the assignee of WORK-434 and Thomas's run on WORK-434 is running
- **WHEN** Brian comments "keep the Markdown toggle" on WORK-434 without changing the recipient's choice
- **THEN** Thomas's run receives "keep the Markdown toggle" at its next step, named as Brian's comment
- **AND** no follow-up run is queued for Thomas while the run runs

#### Scenario: Another agent comments while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Skuddy comments "@Thomas the modal needs it too" on WORK-434
- **THEN** Thomas's run receives the comment at its next step, named as Skuddy's comment

#### Scenario: A reply in another thread of the same issue
- **GIVEN** Thomas's run on WORK-434 is running and Thomas started the thread "Copy button placement"
- **WHEN** Brian replies in that thread
- **THEN** Thomas's run receives the reply at its next step

#### Scenario: A comment with files waits for the run to end
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian comments for Thomas with a screenshot attached
- **THEN** Thomas's run does not receive the comment
- **AND** the comment starts Thomas's follow-up run once the run ends, as before

### Requirement: The composer adds a comment to a running run by default
In the comment composer, a recipient whose run on the issue is running and takes input SHALL default to **Add to current run** in every thread, and **Start after this run**, **Stop and start over** and **Won't trigger** SHALL stay selectable.

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
When a person or another agent changes an issue's status, priority, assignee, title, description, start date or due date, each running run on that issue SHALL receive the change at its next step, naming who changed what from which value to which value.

#### Scenario: Brian raises the priority while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian sets WORK-434's priority from Medium to High
- **THEN** Thomas's run receives "Brian changed the priority from Medium to High" at its next step

#### Scenario: Brian edits the description while Thomas runs
- **GIVEN** Thomas's run on WORK-434 is running
- **WHEN** Brian edits WORK-434's description
- **THEN** Thomas's run receives that Brian changed the description, with the new description

### Requirement: A run never receives its own agent's items
A running run SHALL NOT receive comments, answers or issue changes made by its own agent, and an agent's comment during its run SHALL NOT queue that agent's follow-up run.

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
