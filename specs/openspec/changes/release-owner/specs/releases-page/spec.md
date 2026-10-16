## ADDED Requirements

### Requirement: A release row opens the release's page
A release row SHALL open the release's page. An organisation's name SHALL open its page and a project's name its page.

#### Scenario: v1.1.0 Account and profile
- **WHEN** Brian clicks the row v1.1.0 on Releases
- **THEN** the page of v1.1.0 Account and profile opens

#### Scenario: An organisation and a project
- **WHEN** Brian clicks Rebl Records and, back on Releases, Rebl
- **THEN** Rebl Records' page opens, and then Rebl's page

## REMOVED Requirements

### Requirement: A release row opens a worklane's Timeline at that release
**Reason**: A release row opens the release's own page, as Brian works with a release like a parent issue.

**Migration**: The release page's "Open on Timeline" (`release-page`) opens the same worklane's Timeline with the release in view.
