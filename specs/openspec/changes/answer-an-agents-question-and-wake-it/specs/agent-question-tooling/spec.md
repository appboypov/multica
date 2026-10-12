## ADDED Requirements

### Requirement: Ask a question from the CLI
An agent SHALL ask a question with `multica question ask`, giving the prompt, an optional context line, the answer type with its settings, an optional suggested answer, the member answerers, an optional due date and Blocks work; a question asked during a run SHALL link to that run and its issue without further input.

#### Scenario: Pieter asks a single-choice question
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter runs `multica question ask` with the prompt "Do invoices round per line or per total?", answer type Single choice with options "per line" and "per total", suggested answer "per line" and answerer Brian
- **THEN** WORK-412 shows the question with those options and "per line" marked Suggested
- **AND** the command returns the question's id

#### Scenario: An invalid ask is refused
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter runs `multica question ask` with a suggested answer that is not one of the options
- **THEN** the command fails with an error naming the suggested answer
- **AND** no question is posted

### Requirement: Read answers and end questions from the CLI
An agent SHALL read the answers to its questions, close a question and withdraw a question through `multica question`.

#### Scenario: Pieter reads Brian's answer
- **GIVEN** Brian answered Pieter's question with "per line" and the note "as the old system did"
- **WHEN** Pieter reads the question's answers through `multica question`
- **THEN** the output holds the answer "per line", the note, Brian as who answered and when

#### Scenario: Pieter closes the question
- **GIVEN** Pieter's question is answered
- **WHEN** Pieter closes it through `multica question`
- **THEN** the question is closed on WORK-412

### Requirement: The brief tells only that the tool exists
While questions are on for a run, the run's brief SHALL carry exactly this section, and `multica --help` SHALL list `multica question`:

```text
## Questions

`multica question` asks people one or more questions at the same time and reads their answers.

`multica question --help` lists its commands, answer types and options.
```

No brief, CLI help or skill text MUST say when or why to ask, or when to end a run.

#### Scenario: Pieter's brief
- **GIVEN** questions are on for Pieter's run on WORK-412
- **WHEN** the run starts
- **THEN** its brief holds the Questions section above, word for word
- **AND** `multica --help` lists `multica question`

#### Scenario: No guidance on when to ask
- **GIVEN** questions are on for a run
- **WHEN** the brief, `multica question --help` and the `multica-platform` skill are read
- **THEN** none of them says when or why to ask a question or when to end a run
