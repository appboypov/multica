## ADDED Requirements

### Requirement: The Skills tab filters an agent's skills
An agent's Capabilities > Skills tab SHALL have one search field that keeps, in both the assigned list and the "Inherited from runtime" list, the skills whose name, description or source path holds the search text, without regard to case, and SHALL highlight the match. The section switch SHALL still act on every inherited skill, also the ones the search hides.

#### Scenario: Searching both lists
- **GIVEN** Skuddy has the assigned skills "Review changes" and "Plan sprint" and the inherited skills "Local review" and "Local plan"
- **WHEN** Brian types "plan" in the search field
- **THEN** the tab shows "Plan sprint" and "Local plan"
- **AND** the tab hides "Review changes" and "Local review"

#### Scenario: The section switch acts on hidden skills
- **GIVEN** Brian searched "plan" on Skuddy's Skills tab and the section switch is on
- **WHEN** Brian turns the section switch off
- **THEN** "Local plan" and "Local review" are both off

### Requirement: The Add skill dialog filters the library
The Add skill dialog SHALL have a search field that keeps the library skills whose name or description holds the search text, without regard to case, and SHALL say so when no skill matches.

#### Scenario: Searching the dialog
- **GIVEN** the library holds "madspec-update" and "release-notes", neither assigned to Skuddy
- **WHEN** Brian opens Add skill on Skuddy and types "madspec"
- **THEN** the dialog lists only "madspec-update"
