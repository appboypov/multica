## MODIFIED Requirements

### Requirement: Plan release says what happens when it ships
Plan release SHALL show the read-only line "When its issues are done and merged into {default branch}, it goes on main. After a staging test, beta and then production each start by hand with their own consent of the client or user. It gets its date when it ships." for a repository with a beta environment, and "When its issues are done and merged into {default branch}, it goes on main. After a staging test, production starts by hand with the consent of the client or user. It gets its date when it ships." for a repository without one. The line MUST NOT mention a pass, a tag, a release note or a GitHub release.

#### Scenario: Patch picked
- **GIVEN** the Plan release dialog of appboypov/rebl, whose default branch is `main` and which has no beta environment, with Patch v1.1.1 picked
- **THEN** it shows "When its issues are done and merged into main, it goes on main. After a staging test, production starts by hand with the consent of the client or user. It gets its date when it ships."

#### Scenario: A repository with beta
- **GIVEN** the Plan release dialog of MultiChannable/multichannable, whose default branch is `main` and which has a beta environment
- **THEN** it shows "When its issues are done and merged into main, it goes on main. After a staging test, beta and then production each start by hand with their own consent of the client or user. It gets its date when it ships."
