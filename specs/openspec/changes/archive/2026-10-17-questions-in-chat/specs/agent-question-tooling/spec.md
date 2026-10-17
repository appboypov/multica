## MODIFIED Requirements

### Requirement: Ask a question from the CLI
An agent SHALL ask a question with `multica question ask`, giving the prompt, an optional context line, the answer type with its settings, an optional suggested answer, the member answerers, an optional due date and Blocks work; a question asked during an issue run SHALL link to that run and its issue, and a question asked during a chat turn SHALL link to that run and its chat, without further input.

#### Scenario: Pieter asks a single-choice question
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter runs `multica question ask` with the prompt "Do invoices round per line or per total?", answer type Single choice with options "per line" and "per total", suggested answer "per line" and answerer Brian
- **THEN** WORK-412 shows the question with those options and "per line" marked Suggested
- **AND** the command returns the question's id

#### Scenario: Pieter asks from a chat
- **GIVEN** Pieter is in a turn of Brian's chat
- **WHEN** Pieter runs the same `multica question ask`
- **THEN** Brian's chat shows the question with those options and "per line" marked Suggested
- **AND** the command returns the question's id

#### Scenario: An invalid ask is refused
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter runs `multica question ask` with a suggested answer that is not one of the options
- **THEN** the command fails with an error naming the suggested answer
- **AND** no question is posted

### Requirement: Asking refuses while off
While questions are off for the run an agent is in, `multica question ask` MUST refuse and post nothing; a run that is neither on an issue nor a chat turn always has questions off.

#### Scenario: Pieter tries to ask
- **GIVEN** WORK-420 has Agent questions Off
- **WHEN** Pieter runs `multica question ask` during a run on WORK-420
- **THEN** the command fails and no question is posted

#### Scenario: A run without an issue or chat
- **GIVEN** the workspace Questions default is On
- **WHEN** an agent runs `multica question ask` during a quick-create run, which has no issue and no chat
- **THEN** the command fails and no question is posted
