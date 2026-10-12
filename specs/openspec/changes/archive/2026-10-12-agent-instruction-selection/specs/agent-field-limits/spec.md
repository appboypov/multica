## ADDED Requirements

### Requirement: An agent description has no length limit

The API and the database SHALL accept an agent description of any length.

#### Scenario: A long description through the API

- **GIVEN** a description of 1,000 characters
- **WHEN** an agent is created or updated with that description
- **THEN** the request succeeds and the agent returns the full 1,000 characters

### Requirement: Conversation starters have no length limit and at most three

The API SHALL accept conversation starter labels and prompts of any length, still requiring both and rejecting more than 3 starters.

#### Scenario: A long label and prompt through the API

- **GIVEN** a starter whose label has 120 characters and whose prompt has 5,000
- **WHEN** an agent is saved with that starter
- **THEN** the request succeeds and both are returned whole

#### Scenario: Four starters

- **GIVEN** 4 conversation starters, each with a label and a prompt
- **WHEN** an agent is saved with them
- **THEN** the API rejects it with 400

#### Scenario: An empty label

- **WHEN** an agent is saved with a starter whose label is empty
- **THEN** the API rejects it with 400
