# agent-questions-setting Specification

## Purpose
Lets owners and admins decide per workspace, and members who can edit an issue decide per issue, whether agents may ask questions; agents neither see nor change it.

## Requirements

### Requirement: Workspace Questions default
Owners and admins SHALL set whether agents may ask questions by default in **Settings → Issues & workflow → Questions**, and other members MUST NOT change it; the setting's row SHALL count the open issues that have their own setting.

#### Scenario: Owner turns the default off
- **GIVEN** Brian owns the workspace and the Questions default is On
- **WHEN** Brian turns it off
- **THEN** the Questions default is Off

#### Scenario: A member cannot change it
- **GIVEN** Anouk is a member who is neither owner nor admin
- **WHEN** Anouk opens Settings → Issues & workflow → Questions
- **THEN** she cannot change the switch

#### Scenario: Count of issues with their own setting
- **GIVEN** two open issues are set to Off and one open issue to On
- **WHEN** Brian opens the Questions settings
- **THEN** its row counts 3 open issues with their own setting

### Requirement: Agent questions on an issue
Each issue SHALL have **Agent questions** set to **Workspace default**, **On** or **Off**, changeable at any time by members who can edit the issue; Workspace default SHALL show the default's current value and follow it, and a change SHALL apply from the next run of any agent on that issue. A single run MUST NOT have a setting of its own. Agents MUST NOT see or change an issue's Agent questions.

#### Scenario: Workspace default follows the workspace
- **GIVEN** WORK-420 is on Workspace default and the Questions default is On
- **WHEN** Brian turns the Questions default off
- **THEN** WORK-420's Agent questions row reads Workspace default with Off
- **AND** the next run on WORK-420 has questions off

#### Scenario: A change applies from the next run
- **GIVEN** Pieter is in a run on WORK-420 with questions on
- **WHEN** Brian sets WORK-420 to Off
- **THEN** Pieter's running run is unchanged and the next run of any agent on WORK-420 has questions off

#### Scenario: Back to the default
- **GIVEN** WORK-420 is Off and the Questions default is On
- **WHEN** Brian sets WORK-420 to Workspace default
- **THEN** the next run on WORK-420 has questions on

#### Scenario: An agent cannot see or change it
- **GIVEN** WORK-420 is Off and Pieter is in a run on WORK-420
- **WHEN** Pieter reads WORK-420 and tries to set its Agent questions to On
- **THEN** the issue Pieter reads has no Agent questions
- **AND** his change is refused and WORK-420 stays Off
