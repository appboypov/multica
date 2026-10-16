# Agent active sessions

## Purpose

List an agent's registered terminal sessions and active Multica runs with their work context, keep terminal status and branch current, and remove sessions when they end.

## Requirements

### Requirement: Multica holds each agent's active sessions

Multica SHALL hold the active sessions of each agent: every session its persona command registered and every run of the agent in Multica. Each session SHALL carry its agent, where it runs (a Herdr pane, or a Multica run with its runtime), its folder, its repository, its branch, its issue when it has one, its status and its start time.

#### Scenario: A registered session

- **GIVEN** no Mitchel session runs
- **WHEN** a Mitchel session registers with the daemon in pane `w12:p0`, in a worktree of `appboypov/madspec` on branch `work-1057`
- **THEN** Mitchel's active sessions hold that session with pane `w12:p0`, its folder, `appboypov/madspec` and `work-1057`

#### Scenario: A Multica run

- **GIVEN** no Mitchel session runs
- **WHEN** a run of Mitchel starts on WORK-1057
- **THEN** Mitchel's active sessions hold that run with its runtime, its work folder, its repository, its branch and WORK-1057

### Requirement: Herdr keeps the sessions current

For a session in a Herdr pane, Multica SHALL follow the pane's agent status and the folder's current branch as they change.

#### Scenario: The agent starts working

- **GIVEN** a Mitchel session in pane `w12:p0` that is idle
- **WHEN** Herdr reports the pane's agent as working
- **THEN** Mitchel's active sessions show that session as working

#### Scenario: The branch changes

- **GIVEN** a Mitchel session in pane `w12:p0` on branch `work-1057`
- **WHEN** the session switches its folder to branch `work-1060`
- **THEN** Mitchel's active sessions show that session on `work-1060`

### Requirement: An ended session leaves the list

A session SHALL leave its agent's active sessions once it ends, however it ends: its Herdr pane closes or exits, it crashes, it ends outside Herdr, or its Multica run finishes, fails or is cancelled.

#### Scenario: The pane closes

- **GIVEN** a Mitchel session in pane `w12:p0`
- **WHEN** Brian closes the pane
- **THEN** Mitchel's active sessions no longer hold it

#### Scenario: The session crashes

- **GIVEN** a Mitchel session in pane `w12:p0`
- **WHEN** omp crashes and the pane's process exits
- **THEN** Mitchel's active sessions no longer hold it

#### Scenario: A session outside Herdr

- **GIVEN** a Mitchel session started in a terminal outside Herdr
- **WHEN** the session ends
- **THEN** Mitchel's active sessions no longer hold it

#### Scenario: The run finishes

- **GIVEN** a Mitchel run on WORK-1057
- **WHEN** the run finishes
- **THEN** Mitchel's active sessions no longer hold it

### Requirement: A run is never the daemon's pane

A Multica run MUST NOT be listed as running in the Herdr pane of the daemon that started it, and a Herdr event of that pane MUST NOT change or end a run's session.

#### Scenario: The daemon runs in Herdr

- **GIVEN** the daemon runs in Herdr pane `w75:p0` and a Mitchel run started from it on WORK-1057
- **WHEN** Mitchel's active sessions are listed
- **THEN** the run is listed as a Multica run, not in pane `w75:p0`

#### Scenario: The daemon's pane changes status

- **GIVEN** a Mitchel run started from the daemon in pane `w75:p0`
- **WHEN** Herdr reports pane `w75:p0` as idle
- **THEN** the run's session keeps its own status

### Requirement: An agent's sessions on the command line

`multica agent sessions <agent>` SHALL list the active sessions of that agent with where each runs, its folder, repository, branch, issue and status, and SHALL work from a session on any runtime.

#### Scenario: Two Mitchel sessions

- **GIVEN** a Mitchel session in pane `w12:p0` and a Mitchel run on WORK-1057
- **WHEN** a session runs `multica agent sessions mitchel --output json`
- **THEN** the output lists both, each with where it runs, its folder, repository, branch, issue and status

#### Scenario: No sessions

- **GIVEN** no Guus session runs
- **WHEN** a session runs `multica agent sessions guus`
- **THEN** the output lists no session

### Requirement: A session started by hand registers itself

`multica agent register-session <agent>` SHALL register the session with the given process, folder and Herdr pane through the daemon on its machine, and SHALL fail with a clear message when no daemon runs or the agent does not exist.

Run by hand without `--profile`, `multica agent register-session` and `multica agent sessions` SHALL use the CLI profile whose daemon runs on the machine for that profile's server and workspace: the default profile's, else the one named profile's. With daemons of several named profiles they SHALL fail and ask for `--profile`. An explicit `--profile` and a daemon task keep their own routing.

#### Scenario: Mitchel registers in Herdr

- **GIVEN** the daemon runs and no Mitchel session runs
- **WHEN** the command `mitchel` runs `multica agent register-session mitchel --pid 4242 --cwd ~/Worktrees/das/madspec/work-1057 --pane w12:p0`
- **THEN** Mitchel's active sessions hold that session in pane `w12:p0`

#### Scenario: An unrelated default profile

- **GIVEN** the default CLI profile points to another workspace and runs no daemon
- **AND** the daemon of the CLI profile `desktop-localhost-8080` runs for Mitchel's workspace
- **WHEN** the command `mitchel` runs `multica agent register-session mitchel --pid 4242 --cwd ~/Worktrees/das/madspec/work-1057`
- **THEN** that daemon registers the session in Mitchel's workspace
- **AND** `multica agent sessions mitchel` lists it

#### Scenario: No daemon

- **GIVEN** the daemon does not run
- **WHEN** a session runs `multica agent register-session mitchel --pid 4242 --cwd ~/Worktrees/das/madspec/work-1057`
- **THEN** the command fails and says that the daemon must run
- **AND** Mitchel's active sessions hold nothing new
