## ADDED Requirements

### Requirement: Each row says what the member does
Each Inbox row SHALL show a verb, then the kind of its to-do. The verb of a to-do SHALL follow this table. A row of several to-dos SHALL show the strongest verb of its to-dos, in the order Answer, Pass, Read, Do.

| To-do | Verb |
|---|---|
| Question that is not an Approval question | Answer |
| Mention written by a member | Answer |
| Approval question | Pass |
| Check | Pass |
| Waiting for your review | Pass |
| Assigned, issue In Review | Pass |
| Instruction change | Read |
| Agent blocked, Run failed | Do |
| Autopilot paused, Quota limit reached, Quick create failed | Do |
| Assigned, issue in Todo or Blocked | Do |

#### Scenario: Approval
- **GIVEN** Sarkout asked Brian an Approval question on WORK-412
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-412 shows Pass, then Approval for you

#### Scenario: Question
- **GIVEN** Joeri asked Brian a Single choice question on WORK-413
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-413 shows Answer, then Question for you

#### Scenario: Instruction change
- **GIVEN** Joeri posted an instruction change note for Brian on WORK-414
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-414 shows Read, then Instruction change

#### Scenario: Run failed
- **GIVEN** Joeri's run on WORK-423, which Brian started, failed
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-423 shows Do, then Run failed

#### Scenario: Strongest verb
- **GIVEN** Joeri moved WORK-1655 to Blocked in a run Brian started and Pieter mentions Brian on WORK-1655
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-1655 shows Answer

### Requirement: Waiting on you and Your tasks
The Inbox SHALL show two sections, Waiting on you first and Your tasks second, each with its own count. A row SHALL stand in Your tasks when each of its to-dos is an Assigned to-do on an issue in Todo or Blocked, and in Waiting on you otherwise.

#### Scenario: Own task
- **GIVEN** WORK-974 is assigned to Brian in Todo
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-974 stands under Your tasks

#### Scenario: Decision first
- **GIVEN** WORK-974 is assigned to Brian in Todo and Sarkout asked Brian an Approval question on WORK-412
- **WHEN** Brian opens Inbox
- **THEN** Waiting on you with the row of WORK-412 comes before Your tasks with the row of WORK-974

#### Scenario: Own task with a question
- **GIVEN** WORK-974 is assigned to Brian in Todo and Joeri asks Brian a question on WORK-974
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-974 stands under Waiting on you

#### Scenario: Pass on an assigned issue
- **GIVEN** WORK-1385 is assigned to Brian and moves to In Review
- **WHEN** Brian opens Inbox
- **THEN** the row of WORK-1385 shows Pass and stands under Waiting on you

### Requirement: A finished issue holds no to-dos of its work
When an issue moves to a status of the Done or Closed category, its Mention, Assigned, Waiting for your review, Agent blocked and Run failed to-dos SHALL close. While it stays in such a status, no to-do of these kinds SHALL open on it, whatever the other requirements say. Question and Check to-dos on it SHALL keep their own life cycle.

#### Scenario: Run fails after Done
- **GIVEN** the pull request automation moved WORK-1352 to Done
- **WHEN** Joeri's run on WORK-1352, which Brian started, fails 18 minutes later
- **THEN** Brian has no to-do for WORK-1352

#### Scenario: Done closes
- **GIVEN** Brian has an Agent blocked to-do and a Mention to-do on WORK-1655
- **WHEN** WORK-1655 moves to Done
- **THEN** both to-dos leave Brian's inbox

#### Scenario: Cancelled closes
- **GIVEN** Brian has a Run failed to-do on WORK-1432
- **WHEN** WORK-1432 moves to Cancelled
- **THEN** the to-do leaves Brian's inbox

#### Scenario: Check after Done
- **GIVEN** Sarkout opened a release check for Brian on WORK-1655
- **WHEN** WORK-1655 moves to Done
- **THEN** Brian's Check to-do for WORK-1655 stays

### Requirement: A withdrawn last question closes the Pass on its issue
When a question to a member on an In Review issue is withdrawn and the member has no other open question on that issue, the member's Waiting for your review and Assigned to-dos on that issue SHALL close.

#### Scenario: Last question withdrawn
- **GIVEN** WORK-1385 is In Review, assigned to Brian, with one open question to Brian
- **WHEN** the question is withdrawn
- **THEN** Brian's Assigned to-do for WORK-1385 leaves

#### Scenario: Another question open
- **GIVEN** WORK-1385 is In Review, assigned to Brian, with two open questions to Brian
- **WHEN** one question is withdrawn
- **THEN** Brian's Question to-do for the other question stays on the row of WORK-1385
- **AND** his Assigned to-do for WORK-1385 stays

#### Scenario: Own task stays
- **GIVEN** WORK-974 is assigned to Brian in Todo with one open question to Brian
- **WHEN** the question is withdrawn
- **THEN** Brian's Assigned to-do for WORK-974 stays

### Requirement: To-dos from before the change follow its rules
At release, every open to-do these rules close or never open SHALL close as handled: Mention, Assigned, Waiting for your review, Agent blocked and Run failed to-dos on an issue in the Done or Closed category; Waiting for your review to-dos on an issue with an open linked pull request; Mention to-dos an agent wrote; and Waiting for your review and Assigned to-dos on an In Review issue that opened before the member's last open question on it was withdrawn.

#### Scenario: Failed run on a done issue
- **GIVEN** Brian has a Run failed to-do on WORK-1658, which is Done
- **WHEN** the release runs
- **THEN** the to-do leaves Brian's inbox

#### Scenario: Review with an open pull request
- **GIVEN** Brian has a Waiting for your review to-do on WORK-1675, which is In Review with an open pull request linked
- **WHEN** the release runs
- **THEN** the to-do leaves Brian's inbox

#### Scenario: Agent mention
- **GIVEN** Brian has a Mention to-do on WORK-1196 from a comment an agent wrote
- **WHEN** the release runs
- **THEN** the to-do leaves Brian's inbox

#### Scenario: Withdrawn questions
- **GIVEN** Brian has an Assigned to-do on WORK-1385, which is In Review, and all six questions to Brian on it were withdrawn after it opened
- **WHEN** the release runs
- **THEN** the to-do leaves Brian's inbox

#### Scenario: A person's mention stays
- **GIVEN** Brian has a Mention to-do from Pieter on WORK-418, which is In Progress
- **WHEN** the release runs
- **THEN** the to-do stays

## MODIFIED Requirements

### Requirement: A comment that mentions a member is a to-do
A comment or issue description that @mentions a member SHALL give that member a to-do, unless they wrote it or an agent wrote it. It SHALL leave once the member posts any comment on that issue. A member an agent's comment mentions SHALL get a notice of that comment in Updates.

#### Scenario: Mentioned
- **GIVEN** WORK-418 exists in Brian's workspace
- **WHEN** Pieter posts a comment on WORK-418 that mentions Brian
- **THEN** Brian has a Mention to-do for that comment

#### Scenario: Replied
- **GIVEN** Brian has a Mention to-do on WORK-418
- **WHEN** Brian posts a comment on WORK-418
- **THEN** the to-do leaves

#### Scenario: Mentioned by an agent
- **GIVEN** Brian does not follow WORK-1196
- **WHEN** an agent posts a blocked report on WORK-1196 that mentions Brian
- **THEN** Brian has no to-do for that comment
- **AND** Brian gets a notice of that comment in Updates

### Requirement: An issue the member created that is In Review is a to-do
An issue a member created SHALL be a Waiting for your review to-do while it is In Review, unless the member is its assignee or a pull request linked to it is open. It SHALL leave once the issue leaves In Review or an open pull request is linked to it. Unlinking an unmerged pull request SHALL open it again while the issue is In Review and no other linked pull request is open. While an open question for the member is on the same issue, only the question SHALL show.

#### Scenario: Review
- **GIVEN** Brian created WORK-424
- **WHEN** Joeri moves WORK-424, which Brian created, to In Review
- **THEN** Brian has a Waiting for your review to-do for WORK-424

#### Scenario: Code review is Quality's
- **GIVEN** Brian created WORK-1675 and an open pull request is linked to it
- **WHEN** the Senior Backend agent moves WORK-1675 to In Review
- **THEN** Brian has no to-do for WORK-1675

#### Scenario: Pull request linked later
- **GIVEN** Brian has a Waiting for your review to-do for WORK-424
- **WHEN** an open pull request is linked to WORK-424
- **THEN** the to-do leaves

#### Scenario: Pull request unlinked
- **GIVEN** WORK-424, which Brian created, is In Review with one open pull request linked and no to-do for Brian
- **WHEN** the pull request is unlinked without merge
- **THEN** Brian has a Waiting for your review to-do for WORK-424

#### Scenario: Merge pass question wins
- **GIVEN** WORK-424 is In Review and Brian has a Waiting for your review to-do for it
- **WHEN** Sarkout asks Brian an Approval question on WORK-424
- **THEN** Brian's inbox shows only the Question to-do for WORK-424

#### Scenario: In Progress is no review to-do
- **GIVEN** Brian created WORK-425
- **WHEN** Joeri moves WORK-425, which Brian created, to In Progress
- **THEN** Brian has no to-do for WORK-425

### Requirement: Mark done
A member SHALL be able to mark a row done, which closes at once each of its to-dos that allows Mark done. Check and Instruction change to-dos SHALL close only through their answer and stay on the row. A later event SHALL open a new to-do.

#### Scenario: Mark done
- **GIVEN** Brian has a Mention to-do on WORK-418
- **WHEN** Brian marks its row done
- **THEN** it leaves his inbox and the Inbox number drops by one

#### Scenario: Mentioned again
- **GIVEN** Brian marked his Mention to-do on WORK-418 done
- **WHEN** Pieter mentions Brian again on WORK-418
- **THEN** Brian has a new Mention to-do

#### Scenario: Row with a check
- **GIVEN** the row of WORK-1655 holds Brian's Check to-do and a Mention to-do
- **WHEN** Brian marks the row done
- **THEN** the Mention to-do leaves and the row stays with the Check to-do

### Requirement: Order and count
The Inbox SHALL list one row per issue and one row per to-do without an issue, overdue first, then newest first. A row SHALL be overdue when one of its to-dos is, and as new as its newest to-do. Opening a row SHALL show its strongest to-do, by verb and then by this order, and list the other open to-dos of its issue. The Inbox number SHALL count the rows of the workspace.

#### Scenario: Overdue first
- **GIVEN** Brian has a Mention to-do from today and an overdue Question to-do from last week
- **WHEN** Brian opens Inbox
- **THEN** the Question to-do is first

#### Scenario: Several on one issue
- **GIVEN** WORK-418 is assigned to Brian in Todo and Pieter mentions Brian on it
- **WHEN** Brian opens Inbox
- **THEN** he sees one row for WORK-418, showing Answer
- **AND** opening it shows the Mention to-do and lists the Assigned to-do

#### Scenario: Two failed runs
- **GIVEN** two runs on WORK-1432, which Brian started, failed
- **WHEN** Brian opens Inbox
- **THEN** he sees one row for WORK-1432

#### Scenario: Number counts rows
- **GIVEN** Brian has two Run failed to-dos on WORK-1432 and an Autopilot paused to-do
- **WHEN** Brian opens Inbox
- **THEN** the Inbox number is 2
