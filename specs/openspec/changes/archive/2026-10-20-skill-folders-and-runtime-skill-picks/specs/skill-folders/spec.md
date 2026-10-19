## ADDED Requirements

### Requirement: A folder on a machine feeds the Skills library
The Skills page SHALL let a member add a folder from the machine of their own local runtime, saved per workspace and machine, and SHALL make every direct subfolder of it that holds a `SKILL.md` a library skill with that folder as its source.

#### Scenario: Adding a folder
- **GIVEN** Brian's Mac holds `~/Skills` with the subfolders `madspec-update` and `notes`, and only `madspec-update` holds a `SKILL.md`
- **WHEN** Brian adds `~/Skills` under "Skill folders" on the Skills page
- **THEN** the library lists the skill "madspec-update" with `~/Skills` as its source
- **AND** the library lists no skill for `notes`

#### Scenario: A folder of another member's machine
- **GIVEN** Guus owns the only local runtime on his laptop
- **WHEN** Brian adds a folder through the API for the machine of Guus's runtime
- **THEN** the request is refused
- **AND** no folder is saved

### Requirement: The library follows the folders each time the Skills page opens
Each time the Skills page opens, the system SHALL read every saved folder on its machine and bring the library in line: a new subfolder with a `SKILL.md` SHALL come in, a skill whose subfolder is gone SHALL leave the library and every agent that had it, and a skill whose name another library skill already has SHALL stay out and show "Name taken" under its folder.

#### Scenario: A new subfolder
- **GIVEN** Brian added `~/Skills` earlier
- **WHEN** Brian creates `~/Skills/release-notes/SKILL.md` and opens the Skills page
- **THEN** the library lists "release-notes"

#### Scenario: A gone subfolder
- **GIVEN** the folder skill "release-notes" is assigned to Sarkout
- **WHEN** Brian deletes `~/Skills/release-notes` and opens the Skills page
- **THEN** the library no longer lists "release-notes"
- **AND** Sarkout's Skills tab no longer lists it

#### Scenario: A name already in the library
- **GIVEN** the library holds a skill "review" imported from GitHub
- **WHEN** Brian adds a folder with the subfolder `review` holding a `SKILL.md`
- **THEN** the library keeps only the GitHub skill "review"
- **AND** the folder shows `review` as "Name taken"

### Requirement: A run reads a folder skill from its folder
The system SHALL give an agent a folder skill only when someone adds it with Add skill, and the daemon SHALL read that skill's files from its folder each time a run of the agent starts, for every provider, so an edit to the files counts at the next run.

#### Scenario: An edit counts at the next run
- **GIVEN** the folder skill "madspec-update" is assigned to Skuddy
- **WHEN** Brian edits `~/Skills/madspec-update/SKILL.md` in his editor and a task of Skuddy runs
- **THEN** the run has the edited `SKILL.md`

#### Scenario: A folder skill reaches no agent by itself
- **GIVEN** Brian added a folder with the skill "madspec-update"
- **WHEN** a task of Sarkout runs and nobody added "madspec-update" to Sarkout
- **THEN** the run does not have the folder skill "madspec-update"

### Requirement: A folder skill is read-only in Multica
The system SHALL show a folder skill's files without a way to edit, refresh or delete it in Multica, and SHALL name its folder as its source.

#### Scenario: Opening a folder skill
- **GIVEN** the library holds the folder skill "madspec-update"
- **WHEN** Brian, a workspace owner, opens its page
- **THEN** the page shows its files and `~/Skills/madspec-update` as its source
- **AND** the page offers no edit, refresh or delete

### Requirement: Removing a folder removes its skills
The system SHALL let the member who added a folder, or a workspace owner or admin, remove it, and SHALL then remove its skills from the library and from every agent that had them.

#### Scenario: Removing a folder
- **GIVEN** the folder `~/Skills` feeds "madspec-update", which is assigned to Skuddy
- **WHEN** Brian removes `~/Skills` under "Skill folders"
- **THEN** the library no longer lists "madspec-update"
- **AND** Skuddy's Skills tab no longer lists it
