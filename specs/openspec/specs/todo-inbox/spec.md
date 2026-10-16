# todo-inbox Specification

## Purpose

Lists the actions waiting on a member, closes them when handled, and keeps informational notices in Updates.

## Requirements

### Requirement: A question or approval asked of a member is a to-do
A member SHALL have a to-do for each open question they are an answerer of. It SHALL leave once the member answered it, the question is no longer open, or it is reassigned away from them. A question past its due date SHALL be overdue.

#### Scenario: Asked
- **GIVEN** Brian has no to-dos
- **WHEN** Pieter asks Brian an Approval question on WORK-412
- **THEN** Brian has one Question to-do for it on WORK-412, waiting on Pieter

#### Scenario: Answered
- **GIVEN** Brian has a Question to-do for Pieter's question
- **WHEN** Brian answers the question
- **THEN** the to-do leaves Brian's inbox

#### Scenario: Reassigned
- **GIVEN** Brian has a Question to-do for Pieter's question
- **WHEN** Brian reassigns the question to Anouk
- **THEN** Brian's to-do leaves and Anouk has a Question to-do for it

#### Scenario: Overdue
- **GIVEN** Brian's open question has due date 2026-10-14
- **WHEN** it is 2026-10-15
- **THEN** its to-do is overdue

### Requirement: A comment that mentions a member is a to-do
A comment that @mentions a member SHALL give that member a to-do, unless they wrote it. It SHALL leave once the member posts any comment on that issue.

#### Scenario: Mentioned
- **GIVEN** WORK-418 exists in Brian's workspace
- **WHEN** Pieter posts a comment on WORK-418 that mentions Brian
- **THEN** Brian has a Mention to-do for that comment

#### Scenario: Replied
- **GIVEN** Brian has a Mention to-do on WORK-418
- **WHEN** Brian posts a comment on WORK-418
- **THEN** the to-do leaves

### Requirement: An issue assigned to a member is a to-do in some statuses
An issue assigned to a member SHALL be a to-do while it is in Todo, Back From QA, Blocked or In Review. It SHALL leave once the issue moves to another status or another assignee. A move to another of these statuses SHALL open a new to-do.

#### Scenario: Assigned in Todo
- **GIVEN** WORK-420 is in Todo
- **WHEN** Skuddy assigns WORK-420 in Todo to Brian
- **THEN** Brian has an Assigned to-do for WORK-420

#### Scenario: Moved on
- **GIVEN** Brian has an Assigned to-do for WORK-420 in Todo
- **WHEN** WORK-420 moves to In Progress
- **THEN** the to-do leaves

#### Scenario: Backlog is no to-do
- **GIVEN** WORK-421 is in Backlog
- **WHEN** Skuddy assigns WORK-421 in Backlog to Brian
- **THEN** Brian has no to-do for WORK-421

### Requirement: An agent blocked or a failed run is a to-do of the member the run works for
An issue assigned to an agent that the agent moves to Blocked in a run, and an agent run on an issue that fails, SHALL be a to-do of the member that run works for. It SHALL leave once the issue is no longer Blocked, or an agent run on the issue starts.

#### Scenario: Agent blocked
- **GIVEN** Joeri works on WORK-422 in a run Brian started
- **WHEN** Joeri moves WORK-422 to Blocked
- **THEN** Brian has an Agent blocked to-do for WORK-422, waiting on Joeri

#### Scenario: Agent runs again
- **GIVEN** Brian has an Agent blocked to-do for WORK-422
- **WHEN** Brian's comment wakes Joeri and his run on WORK-422 starts
- **THEN** the to-do leaves

#### Scenario: Run failed
- **GIVEN** Joeri works on WORK-423 in a run Brian started
- **WHEN** Joeri's run on WORK-423, which Brian started, fails
- **THEN** Brian has a Run failed to-do for WORK-423

### Requirement: System problems are to-dos
A paused autopilot, a reached autopilot quota limit and a failed quick create SHALL be to-dos of the members who get their notice today. A paused autopilot SHALL leave once it is resumed, a quota limit once an autopilot run is admitted again, a failed quick create once its retry creates the issue.

#### Scenario: Autopilot paused and resumed
- **GIVEN** Brian's autopilot Daily triage is paused after failing runs
- **WHEN** Brian resumes it
- **THEN** its Autopilot paused to-do leaves

#### Scenario: Quick create failed and retried
- **GIVEN** Brian has a Quick create failed to-do
- **WHEN** Brian retries it and the issue is created
- **THEN** the to-do leaves

### Requirement: An issue the member created that is In Review is a to-do
An issue a member created SHALL be a Waiting for your review to-do while it is In Review, unless the member is its assignee. It SHALL leave once the issue leaves In Review. While an open question for the member is on the same issue, only the question SHALL show.

#### Scenario: Review
- **GIVEN** Brian created WORK-424
- **WHEN** Joeri moves WORK-424, which Brian created, to In Review
- **THEN** Brian has a Waiting for your review to-do for WORK-424

#### Scenario: Merge pass question wins
- **GIVEN** WORK-424 is In Review and Brian has a Waiting for your review to-do for it
- **WHEN** Sarkout asks Brian an Approval question on WORK-424
- **THEN** Brian's inbox shows only the Question to-do for WORK-424

#### Scenario: Waiting For QA is no to-do
- **GIVEN** Brian created WORK-425
- **WHEN** Joeri moves WORK-425, which Brian created, to Waiting For QA
- **THEN** Brian has no to-do for WORK-425

### Requirement: Mark done
A member SHALL be able to mark any of their open to-dos done, which closes it at once. A later event SHALL open a new to-do.

#### Scenario: Mark done
- **GIVEN** Brian has a Mention to-do on WORK-418
- **WHEN** Brian marks it done
- **THEN** it leaves his inbox and the Inbox number drops by one

#### Scenario: Mentioned again
- **GIVEN** Brian marked his Mention to-do on WORK-418 done
- **WHEN** Pieter mentions Brian again on WORK-418
- **THEN** Brian has a new Mention to-do

### Requirement: Order and count
The to-dos SHALL list one row per to-do, overdue first, then newest first. The Inbox number SHALL count the open to-dos of the workspace.

#### Scenario: Overdue first
- **GIVEN** Brian has a Mention to-do from today and an overdue Question to-do from last week
- **WHEN** Brian opens Inbox
- **THEN** the Question to-do is first

#### Scenario: Several on one issue
- **GIVEN** WORK-418 is assigned to Brian in Todo and Pieter mentions Brian on it
- **WHEN** Brian opens Inbox
- **THEN** he sees two rows for WORK-418

### Requirement: Updates holds the notices that ask nothing
The inbox notice list SHALL no longer get question, mention, failed run, paused autopilot, quota and quick create failure notices. Those notices that were not archived SHALL be archived once their to-dos exist.

#### Scenario: Comment on a followed issue
- **GIVEN** Brian follows WORK-419
- **WHEN** Pieter comments on WORK-419, which Brian follows, without mentioning him
- **THEN** Brian gets a New comment notice in Updates and no to-do

#### Scenario: Mention
- **GIVEN** WORK-419 exists in Brian's workspace
- **WHEN** Pieter mentions Brian on WORK-419
- **THEN** Brian gets a Mention to-do and no Mentioned notice
