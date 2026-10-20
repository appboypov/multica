## ADDED Requirements

### Requirement: The releases reference states the phase rules
The built-in releases reference of the multica-platform skill SHALL state that a release goes on `main`, then to beta when its repository has a beta environment, then to production; that getting on `main` starts no deployment; that a release is tested on staging before beta or production; that beta and production each start only by hand, never automatically, after their own explicit consent of the client or user for the tested commit; that a merge, a passed check or test, a staging test or an earlier phase is never a consent; that beta can share production resources and is no isolated test environment; and that `release ship` follows the production start once the release is live.

#### Scenario: An agent reads how to release
- **GIVEN** an agent loads the multica-platform skill
- **WHEN** it opens the releases reference
- **THEN** the reference states each of these rules
- **AND** it no longer tells the agent to publish through the repository's workflow before a recorded consent for that phase

### Requirement: The runtime release brief states the phase rules
The release section of a run's runtime brief SHALL state that no beta or production deployment starts without a staging test of the commit and its own recorded consent, that a merge or a passed test is never that consent, and that `release ship` comes only after the production start.

#### Scenario: A coordination run starts
- **GIVEN** Pieter owns v0.22.3 Sync security
- **WHEN** a coordination run of Pieter on v0.22.3 starts
- **THEN** its runtime brief says that beta and production each start by hand only after a staging test of the commit and their own recorded consent, and that a merge or a passed test is never that consent
