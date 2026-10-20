## ADDED Requirements

### Requirement: The releases reference states the phase rules
The built-in releases reference of the multica-platform skill SHALL state that a release goes on `main`, then to beta when its repository has a beta environment, then to production; that getting on `main` starts no deployment; that a release gets its staging pass before beta or production; that only a workspace owner or admin records a staging pass or a consent, and an agent never does; that beta and production each start only by hand, never automatically, after their own consent; that a merge, a passed check or test, a staging pass or an earlier phase is never a consent; that beta can share production resources and is no isolated test environment; that the agent records the start with `multica release start` and then deploys the way the repository deploys, such as pushing its beta tag or publishing its GitHub release; and that `release ship` follows the production start once the release is live.

#### Scenario: An agent reads how to release
- **GIVEN** an agent loads the multica-platform skill
- **WHEN** it opens the releases reference
- **THEN** the reference states each of these rules
- **AND** it no longer tells the agent to publish through the repository's workflow before the consent for that phase

### Requirement: The runtime release brief states the phase rules
The release section of a run's runtime brief SHALL state that no beta or production deployment starts without the release's staging pass and its own consent, that an agent never records a staging pass or a consent, that a merge or a passed test is never that consent, that a run woken by a consent starts that phase with `multica release start` and deploys it the way the repository deploys, and that `release ship` comes only after the production start.

#### Scenario: A run woken by a consent
- **GIVEN** Pieter owns v0.22.3 Sync security
- **WHEN** Brian's consent for beta queues a coordination run of Pieter on v0.22.3
- **THEN** its runtime brief says to start beta with `multica release start` and deploy it the way MultiChannable/multichannable deploys beta
- **AND** it says that an agent never records a staging pass or a consent
