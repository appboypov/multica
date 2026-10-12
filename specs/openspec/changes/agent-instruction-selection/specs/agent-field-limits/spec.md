## ADDED Requirements

### Requirement: An agent description has no length limit
The API and the database SHALL accept an agent description of any length, and the description inputs SHALL warn above 255 characters without blocking typing or saving.

#### Scenario: A long description through the API
- **WHEN** an agent is created or updated with a description of 1,000 characters
- **THEN** the request succeeds and the agent returns the full 1,000 characters

#### Scenario: A long description in the form
- **WHEN** Brian types a 300-character description in the agent inspector or the create form
- **THEN** all 300 characters stay in the input, the counter warns that 255 is passed, and the description saves

#### Scenario: A description at the threshold
- **WHEN** Brian types a 255-character description
- **THEN** no warning shows

### Requirement: Conversation starters have no length limit and at most three
The API SHALL accept conversation starter labels and prompts of any length, still requiring both and rejecting more than 3 starters; the starter editor SHALL warn above 80 characters for a label and 4000 for a prompt without blocking saving, and the AI agent builder SHALL keep labels and prompts whole, keeping only the first 3 starters.

#### Scenario: A long label and prompt through the API
- **WHEN** an agent is saved with a starter whose label has 120 characters and whose prompt has 5,000
- **THEN** the request succeeds and both are returned whole

#### Scenario: Four starters
- **WHEN** an agent is saved with 4 conversation starters
- **THEN** the API rejects it with 400

#### Scenario: An empty label
- **WHEN** an agent is saved with a starter whose label is empty
- **THEN** the API rejects it with 400

#### Scenario: Warnings in the starter editor
- **WHEN** Brian types an 81-character label and a 4,001-character prompt
- **THEN** the editor warns at both, keeps every character, and the starter saves

#### Scenario: A builder draft with long starters
- **GIVEN** an AI builder draft with 4 starters, the first with a 120-character label
- **WHEN** the draft is applied
- **THEN** the draft holds the first 3 starters and the first label whole

### Requirement: Limits outside agents stay
Text limits outside an agent's own fields MUST stay as they are, the squad description's 255 included.

#### Scenario: A squad description
- **WHEN** Brian types a 300-character description for a squad
- **THEN** the squad form still stops at 255 characters
