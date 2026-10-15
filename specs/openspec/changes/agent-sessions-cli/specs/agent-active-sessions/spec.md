## ADDED Requirements

### Requirement: An agent's sessions on the command line

`multica agent sessions <agent>` SHALL list the active sessions of that agent with where each runs, its folder, repository, branch, issue and status, and SHALL work from a session on any runtime.

#### Scenario: Two Mitchel sessions

- **GIVEN** a Mitchel session in pane `w12:p0` and a Mitchel run on WORK-1057
- **WHEN** a session runs `multica agent sessions mitchel --output json`
- **THEN** the output lists both, each with where it runs, its folder, repository, branch, issue and status

#### Scenario: No sessions

- **GIVEN** no Roos session runs
- **WHEN** a session runs `multica agent sessions roos`
- **THEN** the output lists no session

### Requirement: A session started by hand registers itself

`multica agent register-session <agent>` SHALL register the session with the given process, folder and Herdr pane through the daemon on its machine, and SHALL fail with a clear message when no daemon runs or the agent does not exist.

#### Scenario: Mitchel registers in Herdr

- **GIVEN** the daemon runs and no Mitchel session runs
- **WHEN** the command `mitchel` runs `multica agent register-session mitchel --pid 4242 --cwd ~/Worktrees/das/madspec/work-1057 --pane w12:p0`
- **THEN** Mitchel's active sessions hold that session in pane `w12:p0`

#### Scenario: No daemon

- **GIVEN** the daemon does not run
- **WHEN** a session runs `multica agent register-session mitchel --pid 4242 --cwd ~/Worktrees/das/madspec/work-1057`
- **THEN** the command fails and says that the daemon must run
- **AND** Mitchel's active sessions hold nothing new
