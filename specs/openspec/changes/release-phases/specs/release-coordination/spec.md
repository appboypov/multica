## MODIFIED Requirements

### Requirement: Coordination never ships a release
The platform MUST NOT record a release on main, a staging test, a consent, a beta start or a production start, nor mark a release shipped, because its issues are done or because of a coordination run; each SHALL happen only through its own record with its own checks, and a release SHALL become shipped only through `release ship` with its existing checks.

#### Scenario: Every issue done
- **GIVEN** Pieter owns v1.1.0 Account and profile
- **WHEN** every issue of v1.1.0 is done and its pull requests are merged
- **THEN** v1.1.0 stays unshipped until `release ship` runs for it
- **AND** v1.1.0 is not on main until that is recorded for it

#### Scenario: A coordination run after the staging test
- **GIVEN** Pieter owns v0.22.3 Sync security, which is on main and tested on staging, without a consent for beta
- **WHEN** a coordination run of Pieter on v0.22.3 ends without recording a consent
- **THEN** Beta of v0.22.3 still shows "Needs consent of the client or user"
