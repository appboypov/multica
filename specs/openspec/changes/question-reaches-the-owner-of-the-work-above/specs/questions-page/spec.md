## ADDED Requirements

### Requirement: On my work
The Questions page SHALL have a tab **On my work** listing the open questions whose links or parents have the viewer as member assignee, also when the viewer is not an answerer.

#### Scenario: Brian follows a question below his issue
- **GIVEN** Brian is the assignee of WORK-410 and Pieter asked Anouk a question on its sub-issue WORK-415
- **WHEN** Brian opens On my work
- **THEN** it lists the question, marked "via sub-issue WORK-415"
- **AND** the question is not under Brian's For me and offers Brian no answer controls
