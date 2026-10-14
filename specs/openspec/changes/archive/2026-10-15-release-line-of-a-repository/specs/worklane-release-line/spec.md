## ADDED Requirements

### Requirement: A worklane's Timeline shows its repository's release line
A worklane's Timeline SHALL show its repository's releases from left to right in their order on the line, the first as "Next release" and the others as "Planned", each with its version, its title, its description when it has one, and "No issues of this worklane" while it holds none of the worklane's issues, followed by Plan release. Every worklane of the repository SHALL show the same releases in the same order.

#### Scenario: Three releases on appboypov/rebl
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release, v1.0.1 Membership hardening and v1.1.0 Account and profile, in that order
- **WHEN** Brian opens the Timeline of Fan accounts and push
- **THEN** it shows v1.0.0 First production release as "Next release", then v1.0.1 Membership hardening and v1.1.0 Account and profile as "Planned", each with "No issues of this worklane"
- **AND** Plan release follows v1.1.0

#### Scenario: The same line on every worklane
- **GIVEN** appboypov/rebl has the worklanes `main`, Fan accounts and push and Agenda and publication, and its line holds v1.0.0, v1.0.1 and v1.1.0
- **WHEN** Brian opens the Timeline of `main` and then of Agenda and publication
- **THEN** both show v1.0.0, v1.0.1 and v1.1.0, in that order, with the same titles and descriptions

#### Scenario: A release with a description
- **GIVEN** v1.1.0 Account and profile has the description "Fans make an account, sign in and keep their own profile with a photo."
- **WHEN** Brian opens the Timeline of Fan accounts and push
- **THEN** v1.1.0 shows its title Account and profile and that description

### Requirement: A release shows no date and no goal until it ships
A release on the Timeline SHALL show no date and no goal, and the worklane's header SHALL say "nothing shipped yet" while its repository has shipped no release.

#### Scenario: Planned releases on Fan accounts and push
- **GIVEN** appboypov/rebl's line holds v1.0.0 and v1.0.1, neither shipped
- **WHEN** Brian opens the Timeline of Fan accounts and push
- **THEN** neither release shows a date or a goal
- **AND** the header shows appboypov/rebl and "nothing shipped yet"

### Requirement: A repository that does not release shows so on its worklanes
A worklane of a repository set to "This repository does not release" SHALL show "{repository} does not release, so its worklanes plan no releases." on its Timeline in place of Plan release, and "does not release" in its header.

#### Scenario: appboypov/rebl-site does not release
- **GIVEN** appboypov/rebl-site is set to "This repository does not release"
- **WHEN** Brian opens the Timeline of its worklane `main`
- **THEN** it shows "appboypov/rebl-site does not release, so its worklanes plan no releases." and no Plan release
- **AND** the header shows "does not release"
