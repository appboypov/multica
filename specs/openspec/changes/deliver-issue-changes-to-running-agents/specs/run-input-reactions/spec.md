## ADDED Requirements

### Requirement: The agent marks an item it handles with 👀 and ✅
During its run, an agent SHALL be able to mark an item that run received with 👀 when it saw the item and handles it, and with ✅ once it handled it, through `multica issue react <item> 👀` and `multica issue react <item> ✅`, where `<item>` is the id the item arrived with. An item that is not relevant to the agent gets no mark. Marking the same item twice with the same emoji SHALL change nothing.

#### Scenario: Thomas sees Brian's comment
- **GIVEN** Thomas's running run on WORK-434 received Brian's comment
- **WHEN** Thomas runs `multica issue react <comment id> 👀`
- **THEN** the command succeeds and the comment shows 👀 Thomas

#### Scenario: Thomas handled Brian's comment
- **GIVEN** Brian's comment shows 👀 Thomas
- **WHEN** Thomas runs `multica issue react <comment id> ✅`
- **THEN** the comment shows 👀 Thomas and ✅ Thomas

#### Scenario: Thomas marks ✅ twice
- **GIVEN** Brian's comment shows ✅ Thomas
- **WHEN** Thomas runs `multica issue react <comment id> ✅` again
- **THEN** the command succeeds and the comment still shows one ✅ Thomas

#### Scenario: Another emoji
- **WHEN** Thomas runs `multica issue react <comment id> 👍`
- **THEN** the command fails with "react takes 👀 or ✅"

#### Scenario: Thomas leaves an item that is not for him
- **GIVEN** Thomas's running run on WORK-434 received Brian's reply in Skuddy's thread
- **WHEN** Thomas finds the reply is not relevant to his work and does not mark it
- **THEN** the reply shows no mark of Thomas

#### Scenario: Marks on an edited comment
- **GIVEN** Thomas's run received Brian's comment, marked it ✅, and then received Brian's edit of it
- **WHEN** Thomas marks the edit 👀
- **THEN** the comment shows ✅ Thomas for the first text and 👀 Thomas for the edit

### Requirement: Only the run that received an item marks it
`multica issue react` SHALL succeed only inside a run that received the item, and SHALL fail with a message naming why otherwise.

#### Scenario: The item did not reach this run
- **GIVEN** Joeri's run on WORK-438 is running and Brian's comment on WORK-434 reached only Thomas's run
- **WHEN** Joeri runs `multica issue react <that comment id> ✅`
- **THEN** the command fails with "this run did not receive that item"
- **AND** the comment shows no ✅ Joeri

#### Scenario: Outside a run
- **WHEN** Brian runs `multica issue react <comment id> ✅` in his own terminal
- **THEN** the command fails with "react works only inside an agent's run"

### Requirement: People see 👀 and ✅ where the item is
Each 👀 and ✅ SHALL show with the agent's name on the comment, on the answer in its question card, and on the issue change's row in the issue's activity, and SHALL appear for everyone watching the issue without a reload.

#### Scenario: ✅ on an answer
- **GIVEN** Thomas's run received Brian's answer to Thomas's question on WORK-434
- **WHEN** Thomas marks it ✅
- **THEN** the answer in the question card shows ✅ Thomas

#### Scenario: 👀 on an issue change
- **GIVEN** Thomas's run received Brian's change of WORK-434's priority to High
- **WHEN** Thomas marks it 👀
- **THEN** the activity row "Brian changed the priority from Medium to High" shows 👀 Thomas

#### Scenario: Brian watches the issue
- **GIVEN** Brian has WORK-434 open
- **WHEN** Thomas marks Brian's comment ✅
- **THEN** ✅ Thomas appears on the comment without a reload

### Requirement: A run's end hands on only items without ✅
When a run ends, each item it received with ✅ SHALL start nothing, and each item it received without ✅ SHALL get the handling it would have got without being received: a comment or an edited comment for the agent starts the agent's follow-up run, an answer to the agent's question stays unseen for Wake, and an item that is not for the agent and an issue change start nothing. For a comment the run received more than once, the newest item decides.

#### Scenario: Thomas handled the comment
- **GIVEN** Thomas's run received Brian's comment and marked it ✅
- **WHEN** the run ends
- **THEN** no run is queued for Thomas

#### Scenario: Thomas saw but did not handle the comment
- **GIVEN** Thomas's run received Brian's comment and marked it 👀 only
- **WHEN** the run ends
- **THEN** a follow-up run of Thomas on WORK-434 starts with Brian's comment as its input

#### Scenario: One of two comments handled
- **GIVEN** Thomas's run received two comments of Brian and marked only the first ✅
- **WHEN** the run ends
- **THEN** one follow-up run of Thomas starts with only the second comment as its input

#### Scenario: An unhandled issue change
- **GIVEN** Thomas's run received Brian's priority change and did not mark it ✅
- **WHEN** the run ends
- **THEN** no run is queued for Thomas

#### Scenario: A reply that was not for Thomas
- **GIVEN** Thomas's run received Brian's reply in Skuddy's thread and did not mark it
- **WHEN** the run ends
- **THEN** no run is queued for Thomas

#### Scenario: The edit is not handled
- **GIVEN** Thomas's run received Brian's comment for Thomas and marked it ✅, then received Brian's edit and did not mark it ✅
- **WHEN** the run ends
- **THEN** a follow-up run of Thomas starts with the edited comment as its input

#### Scenario: The run fails
- **GIVEN** Thomas's run received Brian's comment and did not mark it ✅
- **WHEN** the run fails
- **THEN** the comment starts Thomas's follow-up run, as an unhandled comment of a completed run does

### Requirement: The agent brief explains received items
The brief of every run that can take input SHALL tell the agent that comments, edits, answers and issue changes on its issue arrive while it runs as notices, each with its item id; that it marks an item 👀 when it saw it and handles it and ✅ once it handled it; that it leaves an item that is not relevant to it unmarked; and that an item for it without ✅ is handled again after the run.

#### Scenario: Thomas's run starts on omp
- **GIVEN** Thomas runs on the Oh-My-Pi runtime
- **WHEN** his run on WORK-434 starts
- **THEN** its brief names `multica issue react <item> 👀` and `multica issue react <item> ✅` and when to use each

#### Scenario: A run that cannot take input
- **GIVEN** Anouk runs on a runtime that takes no input while it runs
- **WHEN** her run starts
- **THEN** its brief says nothing about received items or `multica issue react`
