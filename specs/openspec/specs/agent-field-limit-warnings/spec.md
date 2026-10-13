# agent-field-limit-warnings Specification

## Purpose

How the web and desktop agent forms treat the old length limits: description and conversation starter inputs warn above their recommended length instead of stopping input or saving, while the starter count and limits outside agents stay.

## Requirements

### Requirement: The description inputs warn above 255 characters

The agent description inputs SHALL warn above 255 characters without blocking typing or saving.

#### Scenario: A long description in the form

- **WHEN** Brian types a 300-character description in the agent inspector or the create form
- **THEN** all 300 characters stay in the input, the counter warns that 255 is passed, and the description saves

#### Scenario: A description at the threshold

- **WHEN** Brian types a 255-character description
- **THEN** no warning shows

### Requirement: The starter editor warns above 80 and 4000 characters

The conversation starter editor SHALL warn above 80 characters for a label and above 4000 for a prompt without blocking typing or saving, and still SHALL allow at most 3 starters, each with a label and a prompt.

#### Scenario: Warnings in the starter editor

- **WHEN** Brian types an 81-character label and a 4,001-character prompt
- **THEN** the editor warns at both, keeps every character, and the starter saves

#### Scenario: An empty label

- **WHEN** a starter's label is empty
- **THEN** the starter cannot be saved

### Requirement: The AI agent builder keeps starter text whole

The AI agent builder SHALL keep conversation starter labels and prompts whole and keep only the first 3 starters.

#### Scenario: A builder draft with long starters

- **GIVEN** an AI builder draft with 4 starters, the first with a 120-character label
- **WHEN** the draft is applied
- **THEN** the draft holds the first 3 starters and the first label whole

### Requirement: Limits outside agents stay

Text limits outside an agent's own fields MUST stay as they are, the squad description's 255 included.

#### Scenario: A squad description

- **WHEN** Brian types a 300-character description for a squad
- **THEN** the squad form still stops at 255 characters
