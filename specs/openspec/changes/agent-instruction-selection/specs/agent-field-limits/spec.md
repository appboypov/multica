## ADDED Requirements

### Requirement: An agent description has no length limit

The API and the database SHALL accept an agent description of any length.

#### Scenario: A long description through the API

- **WHEN** an agent is created or updated with a description of 1,000 characters
- **THEN** the request succeeds and the agent returns the full 1,000 characters

### Requirement: Conversation starters have no length limit and at most three

The API SHALL accept conversation starter labels and prompts of any length, still requiring both and rejecting more than 3 starters.

#### Scenario: A long label and prompt through the API

- **WHEN** an agent is saved with a starter whose label has 120 characters and whose prompt has 5,000
- **THEN** the request succeeds and both are returned whole

#### Scenario: Four starters

- **WHEN** an agent is saved with 4 conversation starters
- **THEN** the API rejects it with 400

#### Scenario: An empty label

- **WHEN** an agent is saved with a starter whose label is empty
- **THEN** the API rejects it with 400
