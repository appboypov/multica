## ADDED Requirements

### Requirement: Nothing about questions while off
While questions are off for a run, its brief, `multica --help` and the skills the run gets MUST NOT mention questions or `multica question`.

#### Scenario: Pieter's run on an Off issue
- **GIVEN** WORK-420 has Agent questions Off
- **WHEN** Pieter's run on WORK-420 starts
- **THEN** its brief has no Questions section
- **AND** `multica --help` does not list `multica question`
- **AND** none of the run's skills mentions questions

### Requirement: Asking refuses while off
While questions are off for the issue an agent runs on, `multica question ask` MUST refuse and post nothing.

#### Scenario: Pieter tries to ask
- **GIVEN** WORK-420 has Agent questions Off
- **WHEN** Pieter runs `multica question ask` during a run on WORK-420
- **THEN** the command fails and no question is posted
