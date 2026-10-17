# agent-question-tooling Specification

## Purpose

Lets an agent ask, read answers and close or withdraw questions through `multica question`, and tells it only that the tool exists.

## Requirements

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

### Requirement: Ask several questions in one CLI call
One `multica question ask` call SHALL post one question or a question set, each question with its own prompt, answer type, settings and suggested answer, sharing the ask's answerers or squad, answer policy, due date and Blocks work.

#### Scenario: Three questions to a squad
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter runs one `multica question ask` with three questions, squad Finance and answer policy Any one
- **THEN** WORK-412 shows one question set card of three questions asked of Finance's human members under Any one
- **AND** the command returns the id of the ask and of each question

### Requirement: Nothing about questions while off
While questions are off for a run, its brief, `multica --help` and the skills the run gets MUST NOT mention questions or `multica question`.

#### Scenario: Pieter's run on an Off issue
- **GIVEN** WORK-420 has Agent questions Off
- **WHEN** Pieter's run on WORK-420 starts
- **THEN** its brief has no Questions section
- **AND** `multica --help` does not list `multica question`
- **AND** none of the run's skills mentions questions

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

### Requirement: Ask from the CLI outside a run
`multica question ask` SHALL ask with a member login outside a run, taking every ask flag it takes in a run, with `--on` naming the issue the question is asked on; without `--on` naming an issue it MUST fail and post nothing.

#### Scenario: Skuddy's session asks Brian
- **GIVEN** a Herdr omp session holds Skuddy's login and has no run
- **WHEN** it runs `multica question ask --on WORK-442 --prompt "Pass the plan for WORK-442?" --type yes_no --answerer Brian`
- **THEN** WORK-442 shows the question for Brian with Skuddy as its asker
- **AND** the command returns the question's id and its ask id

#### Scenario: No --on
- **GIVEN** a Herdr omp session holds Skuddy's login and has no run
- **WHEN** it runs `multica question ask --prompt "Pass the plan?" --type yes_no --answerer Brian`
- **THEN** the command fails with an error that says a question asked outside a run needs `--on` with an issue
- **AND** no question is posted

#### Scenario: The help says how to ask outside a run
- **GIVEN** a terminal with a member login and no run
- **WHEN** the member runs `multica question ask --help`
- **THEN** the help says that outside a run `--on` names the issue the question is asked on
