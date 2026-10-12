## ADDED Requirements

### Requirement: A persona agent selects its whole skill folder
madspec-sync SHALL give each persona agent the selection of its skill folder in the MadSpec checkout, `~/Repos/Ventures/madspec/skills/<skill>`, and the instructions `` `{skill-root}` is <that folder>. Your instructions are the <skill> skill below; follow it. ``, a blank line, then `{selection}`, in place of the written link to the skill.

#### Scenario: Guus
- **WHEN** the sync runs
- **THEN** the agent Guus has the selection `["/Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer"]`
- **AND** its instructions are `` `{skill-root}` is /Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer. Your instructions are the as-ux-designer skill below; follow it. ``, a blank line, then `{selection}`

#### Scenario: A persona agent with a stale selection
- **GIVEN** the agent Guus with another selection
- **WHEN** the sync runs
- **THEN** Guus's selection is its skill folder again

### Requirement: A persona's skill content comes from disk at task start
madspec-sync SHALL keep reading the persona list, models and arguments from `origin/main`, while a persona agent's skill content SHALL be its skill folder in the checkout as it is on disk when a task starts.

#### Scenario: An uncommitted skill edit
- **GIVEN** an uncommitted edit to `skills/as-ux-designer/SKILL.md` in the MadSpec checkout
- **WHEN** a task of Guus starts
- **THEN** Guus's instructions hold the edited `SKILL.md`

#### Scenario: An uncommitted model change
- **GIVEN** an uncommitted edit to the `model` list in `skills/as-ux-designer/customize.toml`
- **WHEN** the sync runs
- **THEN** Guus's model is still the one on `origin/main`
