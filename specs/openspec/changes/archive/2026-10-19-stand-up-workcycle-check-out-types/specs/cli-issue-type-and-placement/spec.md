## RENAMED Requirements

- FROM: `### Requirement: The CLI creates an issue with one of the eight types or none`
- TO: `### Requirement: The CLI creates an issue with one of the eleven types or none`

## MODIFIED Requirements

### Requirement: The CLI creates an issue with one of the eleven types or none
`multica issue create` SHALL take an optional `--type`, one of epic, journey, story, bug, enhancement, refactor, chore, spike, standup, workcycle and checkout, and SHALL refuse any other value, naming the eleven. Without `--type` the issue SHALL have no type.

#### Scenario: A bug
- **WHEN** an agent runs `multica issue create` with the title "Invitation link opens an empty screen" and `--type bug`
- **THEN** the issue is a bug and shows the Bug icon

#### Scenario: A stand-up
- **WHEN** an agent runs `multica issue create` with the title "Stand-up We Work 2026-10-20" and `--type standup`
- **THEN** the issue is a stand-up and shows the Stand-up icon

#### Scenario: No type
- **WHEN** an agent runs `multica issue create` with a title and no `--type`
- **THEN** the issue shows the No type icon

#### Scenario: An unknown type
- **WHEN** an agent runs `multica issue create` with `--type task`
- **THEN** it fails naming epic, journey, story, bug, enhancement, refactor, chore, spike, standup, workcycle and checkout, and creates no issue
