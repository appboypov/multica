## ADDED Requirements

### Requirement: Organisations lists every organisation
Organisations, in the sidebar under Work after Projects, SHALL list every organisation of the workspace by name, each with its number of projects, and SHALL open an organisation's page from its row.

#### Scenario: Organisations with their projects
- **GIVEN** the workspace holds the organisations Rebl Records with 1 project and De App Specialist with 13 projects
- **WHEN** Brian opens Organisations from the sidebar
- **THEN** De App Specialist shows with "13 projects" above Rebl Records with "1 project"
- **AND** the header counts 2 organisations

#### Scenario: An organisation without projects
- **GIVEN** the organisation Buttler holds no project
- **WHEN** Brian opens Organisations
- **THEN** Buttler shows with "0 projects"

#### Scenario: A row opens its organisation
- **GIVEN** Brian is on Organisations
- **WHEN** he clicks Rebl Records
- **THEN** Rebl Records' page opens

### Requirement: New organisation creates an organisation with a name and a key
New organisation on Organisations SHALL ask for a name and a key and SHALL create the organisation with them. A key SHALL be lowercase letters, digits and dashes, starting with a letter or digit. New organisation MUST NOT create an organisation without a name or a key, with a name another organisation of the workspace has, ignoring case, or with a key another organisation has.

#### Scenario: A new organisation
- **GIVEN** no organisation is named Flextender or has the key `flx`
- **WHEN** Brian creates the organisation Flextender with the key `flx` in New organisation
- **THEN** Flextender shows on Organisations with "0 projects"

#### Scenario: A name already taken
- **GIVEN** the organisation Rebl Records exists
- **WHEN** Brian enters "rebl records" in New organisation
- **THEN** no organisation is created
- **AND** the dialog says the name is taken

#### Scenario: A key already taken
- **GIVEN** De App Specialist has the key `das`
- **WHEN** Brian enters the name Das Studio and the key `das` in New organisation
- **THEN** no organisation is created
- **AND** the dialog says the key is taken

#### Scenario: A key that is no folder name
- **WHEN** Brian enters the key `Rebl Records` in New organisation
- **THEN** the dialog says a key is lowercase letters, digits and dashes, and it cannot create the organisation

#### Scenario: No name or no key
- **WHEN** Brian opens New organisation and leaves the name or the key empty
- **THEN** it cannot create the organisation

### Requirement: An organisation's key can change on its page
An organisation's page SHALL show its key and let Brian change it under the rules New organisation applies.

#### Scenario: Changing a key
- **GIVEN** Rebl Records has the key `rr`
- **WHEN** Brian changes it to `rebl` on Rebl Records' page
- **THEN** Rebl Records' page shows the key `rebl`

#### Scenario: A key already taken
- **GIVEN** De App Specialist has the key `das`
- **WHEN** Brian changes Rebl Records' key to `das`
- **THEN** Rebl Records keeps its key and the page says the key is taken

### Requirement: An organisation's page shows its projects
An organisation's page SHALL show the organisation's name, its number of projects, and its projects as Projects shows them, and SHALL offer New project.

#### Scenario: Rebl Records' page
- **GIVEN** Rebl Records holds the project Rebl and De App Specialist holds Multica
- **WHEN** Brian opens Rebl Records' page
- **THEN** it shows the name Rebl Records and "1 project"
- **AND** it lists Rebl and not Multica

#### Scenario: A project made from the organisation's page
- **GIVEN** Brian is on Rebl Records' page
- **WHEN** he creates the project Fan app with New project and the organisation Rebl Records
- **THEN** Fan app shows on Rebl Records' page
- **AND** the page counts "2 projects"

#### Scenario: An organisation that does not exist
- **WHEN** Brian opens the page of an organisation that does not exist
- **THEN** the page says the organisation was not found
