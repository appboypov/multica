## ADDED Requirements

### Requirement: The browser shows the runtime machine's folders
The instructions field SHALL offer a runtime browser, in web and desktop alike, that first lists the skill roots of the agent's runtime machine for its provider and then the folders and files of any folder opened or path typed, as the daemon on that machine reports them; the server MUST NOT read that machine's disk.

#### Scenario: The skill roots of an omp runtime
- **GIVEN** an agent on Brian's online omp runtime
- **WHEN** Brian opens the browser
- **THEN** it lists that machine's omp skill roots, such as `~/.omp/agent/skills`

#### Scenario: Opening a folder
- **GIVEN** the browser lists `~/.omp/agent/skills`
- **WHEN** Brian opens it
- **THEN** the browser lists its entries with their names and whether each is a file or a folder

#### Scenario: A typed path
- **WHEN** Brian types `/Users/codaveto/Repos/Ventures/madspec/skills` in the browser
- **THEN** the browser lists that folder's entries

#### Scenario: A typed path that does not exist
- **WHEN** Brian types a path that does not exist on the runtime machine
- **THEN** the browser says the path does not exist and adds nothing

#### Scenario: The desktop app
- **WHEN** Brian opens the browser in the desktop app
- **THEN** it lists the same runtime machine's skill roots as in the web app

### Requirement: The browser selects several paths at once
The browser SHALL let Brian select several files and folders, across the folders he opens, and add them to the selection in one step, after the paths already selected.

#### Scenario: Two picks from different folders
- **GIVEN** an agent with the selected file `/n/b.md`
- **WHEN** Brian selects the folder `/s/as-ux-designer` and the file `/n/c.md` in the browser and adds them
- **THEN** the selection is `/n/b.md`, `/s/as-ux-designer`, `/n/c.md` in that order

### Requirement: Hidden entries are listed and marked
The browser SHALL list hidden entries (a name starting with `.`), mark them as hidden, and let them be selected.

#### Scenario: A hidden folder
- **GIVEN** a folder that holds `.claude` and `SKILL.md`
- **WHEN** Brian opens it in the browser
- **THEN** `.claude` is listed, marked as hidden, and can be selected

### Requirement: Only the runtime's owner browses it
Only the owner of a runtime SHALL browse its machine; for anyone else the browser SHALL be unavailable and the API SHALL reject a browse request with 403.

#### Scenario: Another member
- **GIVEN** an agent on a public runtime Brian owns, which another member may edit
- **WHEN** that member sends a browse request for that runtime
- **THEN** the API rejects it with 403 and the daemon lists nothing

### Requirement: The browser adds nothing while the runtime is offline
While the agent's runtime is offline the browser SHALL say the runtime is offline and add nothing, and the selected paths SHALL still be removable.

#### Scenario: An offline runtime
- **GIVEN** an agent with one selected path whose runtime is offline
- **WHEN** Brian opens the browser
- **THEN** it says the runtime is offline, lists nothing and adds nothing
- **AND** Brian can still remove the selected path
