## ADDED Requirements

### Requirement: Multica holds each agent's active sessions

Multica SHALL hold the active sessions of each agent: every session its persona command registered and every run of the agent in Multica. Each session SHALL carry its agent, where it runs (a Herdr pane, or a Multica run with its runtime), its folder, its repository, its branch, its issue when it has one, its status and its start time.

#### Scenario: A registered session

- **GIVEN** no Mitchel session runs
- **WHEN** the command `mitchel` registers a session in pane `w12:p0`, in a worktree of `appboypov/madspec` on branch `work-1057`
- **THEN** Mitchel's active sessions hold that session with pane `w12:p0`, its folder, `appboypov/madspec` and `work-1057`

#### Scenario: A Multica run

- **GIVEN** no Mitchel session runs
- **WHEN** a run of Mitchel starts on WORK-1057
- **THEN** Mitchel's active sessions hold that run with its runtime, its work folder, its repository, its branch and WORK-1057

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

### Requirement: Herdr keeps the sessions current

For a session in a Herdr pane, Multica SHALL follow the pane's agent status and the folder's current branch as they change.

#### Scenario: The agent starts working

- **GIVEN** a Mitchel session in pane `w12:p0` that is idle
- **WHEN** Herdr reports the pane's agent as working
- **THEN** `multica agent sessions mitchel` shows that session as working

#### Scenario: The branch changes

- **GIVEN** a Mitchel session in pane `w12:p0` on branch `work-1057`
- **WHEN** the session switches its folder to branch `work-1060`
- **THEN** `multica agent sessions mitchel` shows that session on `work-1060`

### Requirement: An ended session leaves the list

A session SHALL leave its agent's active sessions once it ends, however it ends: its Herdr pane closes or exits, it crashes, it ends outside Herdr, or its Multica run finishes, fails or is cancelled.

#### Scenario: The pane closes

- **GIVEN** a Mitchel session in pane `w12:p0`
- **WHEN** Brian closes the pane
- **THEN** `multica agent sessions mitchel` no longer lists it

#### Scenario: The session crashes

- **GIVEN** a Mitchel session in pane `w12:p0`
- **WHEN** omp crashes and the pane's process exits
- **THEN** `multica agent sessions mitchel` no longer lists it

#### Scenario: A session outside Herdr

- **GIVEN** a Mitchel session started with `mitchel` in a terminal outside Herdr
- **WHEN** the session ends
- **THEN** `multica agent sessions mitchel` no longer lists it

#### Scenario: The run finishes

- **GIVEN** a Mitchel run on WORK-1057
- **WHEN** the run finishes
- **THEN** `multica agent sessions mitchel` no longer lists it

### Requirement: A run is never the daemon's pane

A Multica run MUST NOT be listed as running in the Herdr pane of the daemon that started it, and a Herdr event of that pane MUST NOT change or end a run's session.

#### Scenario: The daemon runs in Herdr

- **GIVEN** the daemon runs in Herdr pane `w75:p0` and a Mitchel run started from it on WORK-1057
- **WHEN** a session runs `multica agent sessions mitchel`
- **THEN** the run is listed as a Multica run, not in pane `w75:p0`

#### Scenario: The daemon's pane changes status

- **GIVEN** a Mitchel run started from the daemon in pane `w75:p0`
- **WHEN** Herdr reports pane `w75:p0` as idle
- **THEN** the run's session keeps its own status
