# issue-templates Specification

## Purpose
Lets Brian create an issue in New issue with a free description from No template, or from one of eleven templates, filling its fields into the template's one Markdown description.

## Requirements

### Requirement: New issue starts from No template or one of eleven templates
New issue SHALL list No template first and then the eleven templates on the left, each with its type icon, its name and its hint, with No template picked when it opens. With No template the issue SHALL have no type and a free description, and the button SHALL say "Create Issue". A picked template SHALL set the issue's type, its fields and the button "Create {template}" in lowercase.

| Template | Hint |
|---|---|
| No template | A title and a free description |
| Epic | An outcome made of stories |
| Journey | One user journey, one change |
| Story | One slice toward an epic |
| Bug | Something broken to reproduce |
| Enhancement | Make a capability better |
| Refactor | Same behaviour, better shape |
| Chore | Upkeep that keeps things running |
| Spike | A question to answer in a time box |
| Stand-up | An organisation's plan for its next releases |
| Workcycle | The approved releases at work |
| Check-out | What a workcycle delivered, for review |

#### Scenario: New issue opens
- **WHEN** Brian clicks New Issue in the sidebar
- **THEN** New issue lists No template, Epic, Journey, Story, Bug, Enhancement, Refactor, Chore, Spike, Stand-up, Workcycle and Check-out, each with its icon and hint
- **AND** No template is picked, the type chip says "No type" and the button says "Create Issue"

#### Scenario: Creating an issue with No template
- **GIVEN** New issue is open with No template picked
- **WHEN** Brian types the title "Check the guest list export" and the description "It misses the last row" and clicks "Create Issue"
- **THEN** the issue is created with no type and the description "It misses the last row"

#### Scenario: Picking Bug
- **WHEN** Brian picks Bug in New issue
- **THEN** the type chip says Bug, the fields are Bug's and the button says "Create bug"

### Requirement: The template's fields
Each template SHALL show its fields, as in this table, and every field SHALL be optional.

| Template | Fields |
|---|---|
| Epic | What we are doing, User stories, Abilities, Acceptance criteria |
| Journey | User journey, Scenarios |
| Story | User story, Currently, Should, Acceptance criteria |
| Bug | Expected result, Actual result, Steps to reproduce, Environment |
| Enhancement | Enhancement, Current behavior, Desired behavior, Acceptance criteria |
| Refactor | Refactor, Behavioral contract, Acceptance criteria |
| Chore | Chore, Baseline, Target state, Acceptance criteria |
| Spike | Spike objective, Research questions, Time box |
| Stand-up | Workcycle, Releases, Since the last workcycle, What waits, What changed, Settled with Brian, Changes to the releases, Client to-dos |
| Workcycle | Releases, Done when |
| Check-out | Delivered against the plan, Not done and why, Questions for Brian, Packages, For the next stand-up |

#### Scenario: Story's fields
- **WHEN** Brian picks Story
- **THEN** the fields are User story, Currently, Should and Acceptance criteria

#### Scenario: Spike's fields
- **WHEN** Brian picks Spike
- **THEN** the fields are Spike objective, Research questions and Time box

#### Scenario: Workcycle's fields
- **WHEN** Brian picks Workcycle
- **THEN** the fields are Releases and Done when

### Requirement: The fields fill the template's one Markdown description
New issue SHALL show the fields, and a Markdown toggle SHALL show the picked template's Markdown instead, each section with its emoji heading and its question as a quote, with what Brian typed in each field placed in its section: a text field as written, a list field one line per item as `- {item}`, a checklist field one line per item as `- [ ] {item}`, and Journey's Scenarios one scenario section per line. A section whose field is empty, or that has no field, SHALL keep only its heading and question. A copy button SHALL copy that Markdown. The created issue SHALL keep only that Markdown as its description and keep no separate fields.

#### Scenario: A story's Markdown
- **GIVEN** Brian picked Story, typed User story "As a fan, I want to change my display name, so that friends recognise me in guest lists." and the Acceptance criteria "Given a signed-in fan on their profile, when they change their display name and save, then the new name shows on their profile"
- **WHEN** he turns on Markdown
- **THEN** it shows "## 👤 User Story", "> Who benefits and what do they want to achieve?" and "- As a fan, I want to change my display name, so that friends recognise me in guest lists."
- **AND** under "## ✅ Acceptance Criteria" it shows "- [ ] Given a signed-in fan on their profile, when they change their display name and save, then the new name shows on their profile"
- **AND** "## 📍 Currently" and "## 🎯 Should" show only their questions

#### Scenario: The issue keeps only the Markdown
- **GIVEN** Brian created REBL-21 from the Story template with User story and Acceptance criteria filled
- **WHEN** he opens REBL-21's description as Markdown
- **THEN** it is the Markdown New issue showed before Create
- **AND** REBL-21 shows no User story or Acceptance criteria field

### Requirement: Brian edits New issue's Markdown by hand
New issue's Markdown SHALL be editable. Once Brian changes it by hand, that Markdown SHALL be the description: the fields SHALL lock, and New issue SHALL keep the edited Markdown until the issue is created.

#### Scenario: A hand edit
- **GIVEN** Brian picked Story, typed the title and User story, and turned on Markdown
- **WHEN** he adds "- Shows in guest lists" under "## 🎯 Should"
- **THEN** the fields lock
- **AND** the created issue's description is that Markdown with the added line

### Requirement: Create stays disabled until the title is filled
New issue SHALL keep its Create button disabled while the title is empty or only spaces, and SHALL enable it once the title is filled, whatever the template's fields hold.

#### Scenario: Only the title filled
- **GIVEN** Brian picked Chore and typed the title "Renew the TLS certificate"
- **WHEN** Chore, Baseline, Target state and Acceptance criteria are empty
- **THEN** "Create chore" is enabled
- **AND** clicking it creates the chore with the Chore template's Markdown as its description

#### Scenario: No title
- **GIVEN** Brian picked Bug and filled Expected result and Actual result
- **WHEN** the title is empty
- **THEN** "Create bug" is disabled

### Requirement: No template asks for implementation details
No template, field, issue view or creation flow MUST ask for implementation details, code or technical design.

#### Scenario: Story in New issue
- **WHEN** Brian picks Story
- **THEN** no field asks for implementation, code or technical design
