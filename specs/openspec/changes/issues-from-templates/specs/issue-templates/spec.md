## ADDED Requirements

### Requirement: New issue starts from No template or one of eight templates
New issue SHALL list No template first and then the eight templates on the left, each with its type icon, its name and its hint, with No template picked when it opens. With No template the issue SHALL have no type and a free description, and the button SHALL say "Create Issue". A picked template SHALL set the issue's type, its fields and the button "Create {template}" in lowercase.

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

#### Scenario: New issue opens
- **WHEN** Brian clicks New Issue in the sidebar
- **THEN** New issue lists No template, Epic, Journey, Story, Bug, Enhancement, Refactor, Chore and Spike, each with its icon and hint
- **AND** No template is picked, the type chip says "No type" and the button says "Create Issue"

#### Scenario: Creating an issue with No template
- **GIVEN** New issue is open with No template picked
- **WHEN** Brian types the title "Check the guest list export" and the description "It misses the last row" and clicks "Create Issue"
- **THEN** the issue is created with no type and the description "It misses the last row"

#### Scenario: Picking Bug
- **WHEN** Brian picks Bug in New issue
- **THEN** the type chip says Bug, the fields are Bug's and the button says "Create bug"

### Requirement: The template's fields are Required or Optional
Each template SHALL show its fields, each marked Required or Optional, as in this table.

| Template | Required | Optional |
|---|---|---|
| Epic | What we are doing, Abilities, Acceptance criteria | User stories |
| Journey | User journey, Scenarios | |
| Story | User story, Acceptance criteria | Currently, Should |
| Bug | Expected result, Actual result | Steps to reproduce, Environment |
| Enhancement | Enhancement, Acceptance criteria | Current behavior, Desired behavior |
| Refactor | Refactor, Behavioral contract | Acceptance criteria |
| Chore | Chore, Acceptance criteria | Baseline, Target state |
| Spike | Spike objective, Time box | Research questions |

#### Scenario: Story's fields
- **WHEN** Brian picks Story
- **THEN** the fields are User story (Required), Currently (Optional), Should (Optional) and Acceptance criteria (Required)

#### Scenario: Spike's fields
- **WHEN** Brian picks Spike
- **THEN** the fields are Spike objective (Required), Research questions (Optional) and Time box (Required)

### Requirement: The fields fill the template's one Markdown description
The Markdown tab SHALL show the picked template's Markdown, each section with its emoji heading and its question as a quote, with what Brian typed in each field placed in its section: a text field as written, a list field one line per item as `- {item}`, a checklist field one line per item as `- [ ] {item}`, and Journey's Scenarios one scenario section per line. A section whose field is empty, or that has no field, SHALL keep only its heading and question. The created issue SHALL keep only that Markdown as its description and keep no separate fields.

#### Scenario: A story's Markdown
- **GIVEN** Brian picked Story, typed User story "As a fan, I want to change my display name, so that friends recognise me in guest lists." and the Acceptance criteria "Given a signed-in fan on their profile, when they change their display name and save, then the new name shows on their profile"
- **WHEN** he opens the Markdown tab
- **THEN** it shows "## 👤 User Story", "> Who benefits and what do they want to achieve?" and "- As a fan, I want to change my display name, so that friends recognise me in guest lists."
- **AND** under "## ✅ Acceptance Criteria" it shows "- [ ] Given a signed-in fan on their profile, when they change their display name and save, then the new name shows on their profile"
- **AND** "## 📍 Currently" and "## 🎯 Should" show only their questions

#### Scenario: The issue keeps only the Markdown
- **GIVEN** Brian created REBL-21 from the Story template with User story and Acceptance criteria filled
- **WHEN** he opens REBL-21's description as Markdown
- **THEN** it is the Markdown the Markdown tab showed before Create
- **AND** REBL-21 shows no User story or Acceptance criteria field

### Requirement: Create stays disabled until the title and every required field are filled
New issue SHALL keep its Create button disabled while the title or any Required field of the picked template is empty or only spaces.

#### Scenario: A required field empty
- **GIVEN** Brian picked Story and typed the title "A fan changes their display name" and User story
- **WHEN** Acceptance criteria is empty
- **THEN** "Create story" is disabled

#### Scenario: Optional fields empty
- **GIVEN** Brian picked Story, typed the title, User story and Acceptance criteria, and left Currently and Should empty
- **THEN** "Create story" is enabled

#### Scenario: No title
- **GIVEN** Brian picked Bug and filled Expected result and Actual result
- **WHEN** the title is empty
- **THEN** "Create bug" is disabled

### Requirement: No template asks for implementation details
No template, field, issue view or creation flow MUST ask for implementation details, code or technical design, and New issue SHALL say under the fields "These fields fill the {template} template's Markdown. The issue keeps only that Markdown as its description; code and technical design go in the change."

#### Scenario: Story in New issue
- **WHEN** Brian picks Story
- **THEN** no field asks for implementation, code or technical design
- **AND** under the fields it says "These fields fill the Story template's Markdown. The issue keeps only that Markdown as its description; code and technical design go in the change."
