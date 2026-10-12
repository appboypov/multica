## ADDED Requirements

### Requirement: Questions on parents
A parent issue and a project SHALL list in their Questions property the questions of everything below them, each marked with the link it came from, such as "via sub-issue WORK-415", and count them in their open count.

#### Scenario: Parent issue
- **GIVEN** Pieter asked Anouk an open question on WORK-415, a sub-issue of WORK-410
- **WHEN** Brian opens WORK-410
- **THEN** its Questions property lists the question marked "via sub-issue WORK-415"
- **AND** its open count includes it

#### Scenario: Project
- **GIVEN** WORK-410 is in project Agent runtime and WORK-415 has an open question
- **WHEN** Brian opens project Agent runtime
- **THEN** its Questions property lists the question marked "via sub-issue WORK-415"

#### Scenario: Question on a comment
- **GIVEN** Pieter asked a question linked only to a comment on WORK-412
- **WHEN** Brian opens WORK-412
- **THEN** its Questions property lists the question marked "via comment"
