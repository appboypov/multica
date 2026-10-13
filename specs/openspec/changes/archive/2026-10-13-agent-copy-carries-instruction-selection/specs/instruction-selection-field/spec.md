## MODIFIED Requirements

### Requirement: A duplicate carries the selection

Duplicating an agent, with Duplicate in web or desktop or with `multica agent copy`, SHALL copy its `instruction_selection` in order with its `instructions` when the owner of its runtime duplicates it and the copy stays on that runtime; any other duplicate SHALL be made with an empty selection, and `multica agent copy` SHALL then print one line on stderr naming why, without changing its stdout.

#### Scenario: Duplicating a persona agent

- **GIVEN** an agent with instructions `{selection}` and one selected folder
- **WHEN** its runtime's owner duplicates it
- **THEN** the new agent has the same instructions and the same selected folder

#### Scenario: Another member duplicates it

- **GIVEN** an agent with instructions `{selection}` and one selected folder, on a public runtime Brian owns
- **WHEN** another member duplicates it
- **THEN** the new agent has the same instructions and no selected folder

#### Scenario: The owner copies it with the CLI

- **GIVEN** an agent with instructions `{selection}` and two selected folders, on a runtime Brian owns
- **WHEN** Brian runs `multica agent copy` on it without `--runtime-id`
- **THEN** the new agent has the same instructions and the same two folders in the same order
- **AND** stderr holds no note about the selection

#### Scenario: A CLI copy to another runtime

- **GIVEN** an agent with instructions `{selection}` and one selected folder, on a runtime Brian owns
- **WHEN** Brian runs `multica agent copy` on it with `--runtime-id` set to another runtime
- **THEN** the new agent has the same instructions and no selected folder
- **AND** stderr holds one line saying the selected files and folders were left behind because they belong to the source runtime's machine
- **AND** stdout is the same `Agent copied` line, or the same JSON with `--output json`, as a copy of an agent without a selection

#### Scenario: Another member copies it with the CLI

- **GIVEN** an agent with instructions `{selection}` and one selected folder, on a public runtime Brian owns
- **WHEN** another member runs `multica agent copy` on it without `--runtime-id`
- **THEN** the copy is created with the same instructions and no selected folder
- **AND** stderr holds one line saying the selected files and folders were left behind because only the runtime's owner can copy them

#### Scenario: A CLI copy of an agent without a selection

- **GIVEN** an agent with no selected paths
- **WHEN** anyone runs `multica agent copy` on it, on its runtime or another
- **THEN** the new agent has no selected paths
- **AND** stderr holds no note about the selection

#### Scenario: The CLI cannot tell who owns the runtime

- **GIVEN** an agent with one selected folder
- **WHEN** `multica agent copy` cannot read the source runtime's owner or the caller's own account
- **THEN** no agent is created
- **AND** the command fails with that error
