## ADDED Requirements

### Requirement: The agent page shows its active sessions

An agent's page SHALL show the agent's active sessions, each with where it runs, its folder, repository, branch, issue and status, and SHALL show that the agent has none when no session runs.

#### Scenario: Two sessions

- **GIVEN** a Mitchel session in pane `w12:p0` and a Mitchel run on WORK-1057
- **WHEN** Brian opens Mitchel's page
- **THEN** it shows both sessions, each with where it runs, its folder, repository, branch, issue and status

#### Scenario: No sessions

- **GIVEN** no Roos session runs
- **WHEN** Brian opens Roos's page
- **THEN** it shows that Roos has no active sessions

#### Scenario: A session ends

- **GIVEN** Brian has Mitchel's page open with the session in pane `w12:p0` on it
- **WHEN** that pane closes
- **THEN** the page no longer shows that session
