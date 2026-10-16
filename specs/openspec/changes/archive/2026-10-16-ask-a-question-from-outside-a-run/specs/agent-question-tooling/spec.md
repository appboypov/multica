## ADDED Requirements

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
