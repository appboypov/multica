# instruction-selection-field Specification

## Purpose

The instructions field on web and desktop: one field for the instructions tab, the create panel and the create dialog that holds the instructions text, the selected paths as chips and the `{selection}` placeholder, editable by the owner of the agent's runtime and read-only for anyone else.

## Requirements

### Requirement: One instructions field everywhere
The instructions tab, the create panel and the create dialog SHALL use the same instructions field: the instructions text, the runtime browser, and the selected paths as chips in selection order, each removable by the runtime's owner.

#### Scenario: Chips in the instructions tab
- **GIVEN** an agent with two selected paths
- **WHEN** Brian opens its instructions tab
- **THEN** both paths show as chips in selection order

#### Scenario: Removing a chip
- **GIVEN** an agent with two selected paths
- **WHEN** Brian removes the first chip and saves
- **THEN** the agent's selection holds only the second path

#### Scenario: Selecting while creating an agent
- **WHEN** Brian creates an agent in the create panel or the create dialog and selects a folder in its instructions field
- **THEN** the created agent has that folder as its selection

#### Scenario: A create draft keeps its selection
- **GIVEN** an unsaved create draft with one selected path
- **WHEN** Brian leaves the create page and comes back
- **THEN** the draft still has that path

### Requirement: The first selection adds the placeholder
When paths are added to an empty selection and the text holds no `{selection}`, the field SHALL insert `<SystemIdentity>`, `{selection}` and `</SystemIdentity>`, each on its own line, at the cursor.

#### Scenario: A first selection into typed text
- **GIVEN** instructions `Follow this.` with the cursor after the period, and no selection
- **WHEN** Brian adds a folder through the browser
- **THEN** the text becomes `Follow this.<SystemIdentity>`, then `{selection}` and `</SystemIdentity>` on their own lines

#### Scenario: A later selection
- **GIVEN** instructions with `{selection}` and one selected path
- **WHEN** Brian adds another path
- **THEN** the text is unchanged

### Requirement: The field warns when text and selection do not match
The field SHALL warn that the selection is not loaded when paths are selected and the text holds no `{selection}`, and SHALL warn that `{selection}` stays empty when the text holds `{selection}` and no path is selected; neither warning SHALL block saving.

#### Scenario: Paths without the placeholder
- **GIVEN** one selected path
- **WHEN** Brian deletes `{selection}` from the text
- **THEN** the field warns that the selection is not loaded
- **AND** saving still succeeds

#### Scenario: The placeholder without paths
- **GIVEN** instructions with `{selection}`
- **WHEN** Brian removes the last chip
- **THEN** the field warns that `{selection}` stays empty
- **AND** saving still succeeds

#### Scenario: Text and selection match
- **GIVEN** instructions with `{selection}` and one selected path
- **WHEN** Brian views the field
- **THEN** no selection warning shows

### Requirement: Others see the selection read-only

Anyone who may see an agent but does not own its runtime SHALL see its selected paths as chips without a control to remove them or add to them.

#### Scenario: Another member sees the selection

- **GIVEN** an agent on Brian's runtime with a selected folder
- **WHEN** another member who may see the agent opens its instructions
- **THEN** they see the selected folder's path without a control to change or remove it

#### Scenario: Another member edits the text

- **GIVEN** an agent on a public runtime Brian owns, with one selected path, which another member may edit
- **WHEN** that member changes the instructions text and saves
- **THEN** the save succeeds and the selection is unchanged

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

### Requirement: The AI agent builder never sets a selection

The AI agent builder MUST NOT set or change an agent's `instruction_selection`.

#### Scenario: A builder draft

- **WHEN** the AI agent builder drafts and creates an agent
- **THEN** the created agent's `instruction_selection` is `[]`

### Requirement: A selection failure shows its reason

A task that failed with `instruction_selection_error` SHALL show the failure label for an instruction selection error and the task's error, in web and desktop and in every supported language.

#### Scenario: The failure in the task list

- **GIVEN** a task that failed with `instruction_selection_error`
- **WHEN** Brian opens the agent's activity
- **THEN** the task shows the failure label for an instruction selection error and the error with the path
