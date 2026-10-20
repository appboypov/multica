## ADDED Requirements

### Requirement: A release page shows its phases and what each still needs
A release page SHALL show the phases On main, Passed on staging, Beta when its repository has a beta environment, and Production, in that order. A reached phase SHALL show who recorded it and when, and Beta and Production SHALL show their consent and their start each with who gave it and when. A phase not reached yet SHALL show the first thing it still needs: "Needs to go on main", "Needs a staging pass", "Waits for beta", "Needs consent" or "Ready to start".

#### Scenario: MultiChannable 0.22.3 on main
- **GIVEN** v0.22.3 Sync security of MultiChannable/multichannable, which has a beta environment, is on main and has no staging pass
- **WHEN** Brian opens its page
- **THEN** it shows On main with its date and who recorded it, then Passed on staging, Beta and Production each with "Needs a staging pass"

#### Scenario: Beta ready, production waits
- **GIVEN** v0.22.3 Sync security has its staging pass and Brian's consent for beta
- **WHEN** Brian opens its page
- **THEN** Beta shows "Ready to start" with Brian's consent
- **AND** Production shows "Waits for beta"

#### Scenario: A planned release
- **GIVEN** v1.2.0 Notifications of appboypov/rebl is planned and appboypov/rebl has no beta environment
- **WHEN** Brian opens its page
- **THEN** it shows On main, Passed on staging and Production each with "Needs to go on main", and no Beta
