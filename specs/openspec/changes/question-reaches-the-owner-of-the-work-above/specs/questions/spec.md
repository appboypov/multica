## ADDED Requirements

### Requirement: Link a question to any workspace object
A question SHALL be able to link to an issue, a sub-issue, a project, a comment, an agent run or another workspace object, and to more than one of them; the asking agent SHALL be able to add links after the first.

#### Scenario: Asked on a comment
- **GIVEN** Pieter runs on WORK-415
- **WHEN** Pieter asks a question linked to a comment on WORK-415
- **THEN** the question links to that comment

#### Scenario: Adding a link later
- **GIVEN** Pieter's open question links to WORK-415
- **WHEN** Pieter adds a link to project Agent runtime through `multica question`
- **THEN** the question links to WORK-415 and to project Agent runtime, with WORK-415 still its first link

### Requirement: Parents of a link
Every object above a question's link SHALL be a parent of the question: a comment's issue, an issue's parent issues up the chain, and their project.

#### Scenario: Sub-issue in a project
- **GIVEN** WORK-415 is a sub-issue of WORK-410, which is in project Agent runtime
- **WHEN** Pieter asks a question linked to WORK-415
- **THEN** WORK-410 and project Agent runtime are its parents
