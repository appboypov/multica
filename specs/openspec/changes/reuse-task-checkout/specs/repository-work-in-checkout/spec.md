## ADDED Requirements

### Requirement: An agent can request its prepared checkout

For an issue task working in its repository checkout with Work in the project folder on, `multica repo checkout owner/name` SHALL print that checkout's path without creating a worktree or changing the task's branch or files.

#### Scenario: Request the checkout on an issue branch

- **GIVEN** the active issue task works in its repository checkout on its issue branch
- **AND** the agent has uncommitted edits there
- **WHEN** the agent runs `multica repo checkout owner/name` for that repository
- **THEN** the command prints the checkout path
- **AND** no worktree is created
- **AND** the issue branch and edits stay unchanged

#### Scenario: Request the checkout on a pull request branch

- **GIVEN** the active issue task works in its repository checkout on its open pull request's branch
- **WHEN** the agent runs `multica repo checkout owner/name` for that repository
- **THEN** the command prints the checkout path
- **AND** no worktree is created
- **AND** the pull request branch stays unchanged
