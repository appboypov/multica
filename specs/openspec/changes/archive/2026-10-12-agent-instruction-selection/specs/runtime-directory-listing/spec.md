## ADDED Requirements

### Requirement: The API lists a runtime machine's folders through its daemon

The API SHALL take a listing request for a runtime, with a path or without one, and return what the daemon on that runtime's machine reports: without a path, the skill roots of the runtime's provider that exist on that machine; with a path, the entries of that folder, each with its name, its absolute path, whether it is a file or a folder, and whether it is hidden. The server MUST NOT read that machine's disk.

#### Scenario: The skill roots of an omp runtime

- **GIVEN** Brian's online omp runtime, whose machine holds `~/.omp/agent/skills`
- **WHEN** Brian requests a listing without a path
- **THEN** the result holds the folder `/Users/codaveto/.omp/agent/skills`

#### Scenario: A folder's entries

- **GIVEN** the folder `/s` on Brian's runtime machine, holding the folder `as-ux-designer`, the file `README.md` and the folder `.claude`
- **WHEN** Brian requests a listing of `/s`
- **THEN** the result holds `as-ux-designer` as a folder, `README.md` as a file and `.claude` as a hidden folder, each with its absolute path

#### Scenario: A path under the home folder

- **WHEN** Brian requests a listing of `~/notes` on his runtime
- **THEN** the daemon lists `/Users/codaveto/notes` and the result names that absolute path

#### Scenario: A linked folder

- **GIVEN** `/s/link`, a link to a folder
- **WHEN** Brian requests a listing of `/s`
- **THEN** the result holds `link` as a folder

#### Scenario: A path that does not exist

- **WHEN** Brian requests a listing of a path that does not exist on the runtime machine
- **THEN** the request fails with an error that says the path does not exist

### Requirement: Only the runtime's owner lists its machine

Only the owner of a runtime SHALL request a listing of its machine; the API SHALL reject anyone else's request with 403 and the daemon SHALL list nothing for it.

#### Scenario: Another member

- **GIVEN** a public runtime Brian owns, on which another member may run agents
- **WHEN** that member requests a listing of that runtime
- **THEN** the API rejects it with 403 and the daemon lists nothing

### Requirement: An offline runtime lists nothing

The API SHALL reject a listing request for an offline runtime with 503.

#### Scenario: An offline runtime

- **GIVEN** Brian's runtime is offline
- **WHEN** Brian requests a listing of it
- **THEN** the API rejects it with 503 and no request reaches the daemon
