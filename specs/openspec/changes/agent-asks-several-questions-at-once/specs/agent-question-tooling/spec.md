## ADDED Requirements

### Requirement: Ask several questions in one CLI call
One `multica question ask` call SHALL post one question or a question set, each question with its own prompt, answer type, settings and suggested answer, sharing the ask's answerers or squad, answer policy, due date and Blocks work.

#### Scenario: Three questions to a squad
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter runs one `multica question ask` with three questions, squad Finance and answer policy Any one
- **THEN** WORK-412 shows one question set card of three questions asked of Finance's human members under Any one
- **AND** the command returns the id of the ask and of each question
