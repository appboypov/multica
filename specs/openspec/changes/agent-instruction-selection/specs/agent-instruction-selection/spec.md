## ADDED Requirements

### Requirement: An agent keeps an ordered selection of paths
An agent SHALL hold `instruction_selection`, an ordered list of absolute paths on its runtime machine, beside its `instructions`, empty by default, and the agent API SHALL return it in the order it was saved.

#### Scenario: A selection round trip
- **GIVEN** an agent on Brian's runtime
- **WHEN** Brian saves `instruction_selection` as `["/Users/codaveto/notes/b.md", "/Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer"]`
- **THEN** reading the agent returns that list in that order

#### Scenario: A new agent without a selection
- **WHEN** an agent is created without `instruction_selection`
- **THEN** reading the agent returns `instruction_selection` as `[]`

#### Scenario: An update that leaves the selection out
- **GIVEN** an agent with a selection of two paths
- **WHEN** an update changes only its name
- **THEN** the agent still has the same two paths in the same order

#### Scenario: A relative path
- **WHEN** an update sets `instruction_selection` to `["notes/b.md"]`
- **THEN** the API rejects it with 400 and the stored selection is unchanged

### Requirement: The server stores paths, never file content
The server MUST NOT read any file of a selection and MUST NOT store any content of it; it SHALL store and send only the paths.

#### Scenario: The stored agent holds paths only
- **GIVEN** an agent whose selection is a folder of three files
- **WHEN** a task of that agent runs
- **THEN** the agent's database row and every server response about it hold the folder's path and none of the three files' content

### Requirement: Only the runtime's owner changes a selection
Only the owner of an agent's runtime SHALL create an agent with a selection or change an agent's selection; anyone else who may see the agent SHALL see its selected paths without changing them.

#### Scenario: The runtime's owner changes the selection
- **GIVEN** an agent on a runtime Brian owns
- **WHEN** Brian adds a path to its selection
- **THEN** the update succeeds

#### Scenario: Another member tries to change the selection
- **GIVEN** an agent on a public runtime Brian owns, which another workspace member may edit
- **WHEN** that member sends an update that changes `instruction_selection`
- **THEN** the API rejects it with 403 and the selection is unchanged

#### Scenario: Another member sees the selection
- **GIVEN** an agent on Brian's runtime with a selected folder
- **WHEN** another member who may see the agent opens its instructions
- **THEN** they see the selected folder's path without a control to change or remove it

### Requirement: A duplicate carries the selection
Duplicating an agent SHALL copy its `instruction_selection` with its `instructions`.

#### Scenario: Duplicating a persona agent
- **GIVEN** an agent with instructions `{selection}` and one selected folder
- **WHEN** its owner duplicates it
- **THEN** the new agent has the same instructions and the same selected folder

### Requirement: The AI agent builder never sets a selection
The AI agent builder MUST NOT set or change an agent's `instruction_selection`.

#### Scenario: A builder draft
- **WHEN** the AI agent builder drafts and creates an agent
- **THEN** the created agent's `instruction_selection` is `[]`
