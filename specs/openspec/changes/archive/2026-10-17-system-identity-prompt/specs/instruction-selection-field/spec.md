## MODIFIED Requirements

### Requirement: The first selection adds the placeholder
When paths are added to an empty selection and the text holds no `{selection}`, the field SHALL insert `<SystemIdentity>`, `{selection}` and `</SystemIdentity>`, each on its own line, at the cursor.

#### Scenario: A first selection into typed text
- **GIVEN** instructions `Follow this.` with the cursor after the period, and no selection
- **WHEN** Brian adds a folder through the browser
- **THEN** the text becomes `Follow this.<SystemIdentity>`, then `{selection}` and `</SystemIdentity>` on their own lines

#### Scenario: A later selection
- **GIVEN** instructions with `{selection}` and one selected path
- **WHEN** Brian adds another path
- **THEN** the text is unchanged
