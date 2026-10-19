## RENAMED Requirements

- FROM: `### Requirement: New issue starts from No template or one of eight templates`
- TO: `### Requirement: New issue starts from No template or one of eleven templates`

## MODIFIED Requirements

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
