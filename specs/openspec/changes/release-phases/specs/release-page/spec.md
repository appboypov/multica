## ADDED Requirements

### Requirement: A release page shows its phases and what each still needs
A release page SHALL show the phases On main, Tested on staging, Beta when its repository has a beta environment, and Production, in that order. A reached phase SHALL show its date and who recorded it, Tested on staging also its commit and result link, and Beta and Production also their consent with who agreed, and their start with who started it and the commit. A phase not reached yet SHALL show the first thing it still needs: "Needs to go on main", "Needs a staging test", "Waits for beta", "Needs consent of the client or user" or "Ready to start by hand".

#### Scenario: MultiChannable 0.22.3 on main
- **GIVEN** v0.22.3 Sync security of MultiChannable/multichannable, which has a beta environment, is on main and has no staging test
- **WHEN** Brian opens its page
- **THEN** it shows On main with its date and who recorded it, then Tested on staging "Needs a staging test", Beta "Needs a staging test" and Production "Needs a staging test"

#### Scenario: Beta ready, production waits
- **GIVEN** v0.22.3 Sync security has its staging test of commit 24f708b and Milad's consent for beta of 24f708b
- **WHEN** Brian opens its page
- **THEN** Beta shows "Ready to start by hand" with Milad's consent
- **AND** Production shows "Waits for beta"

#### Scenario: A planned release
- **GIVEN** v1.2.0 Notifications of appboypov/rebl is planned and appboypov/rebl has no beta environment
- **WHEN** Brian opens its page
- **THEN** it shows On main "Needs to go on main", Tested on staging "Needs to go on main" and Production "Needs to go on main", and no Beta
