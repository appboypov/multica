## ADDED Requirements

### Requirement: List shows a section per release with its issues of this worklane
A worklane's List SHALL show one section per release of its repository's line, in the line's order, each with the release's version and state, its title, "Set by hand" when its version was set by hand, "{done} of {total} done" over this worklane's issues in it or "No issues of this worklane", the release's menu and its description when it has one, followed by its issues of this worklane as rows of Multica's issue list with their type icon.

#### Scenario: Two releases on Fan accounts and push
- **GIVEN** appboypov/rebl's line holds v1.0.0 First production release, the next release, with REBL-21 (Todo) and REBL-22 (Done) of Fan accounts and push, and v1.1.0 Account and profile, set by hand, with the description "Fans make an account, sign in and keep their own profile with a photo." and none of its issues
- **WHEN** Brian opens the List of Fan accounts and push
- **THEN** the first section shows v1.0.0 as the next release, First production release, "1 of 2 done", the release's menu, and the rows REBL-21 and REBL-22
- **AND** the second shows v1.1.0 as planned, Account and profile, "Set by hand", "No issues of this worklane", the release's menu and its description

#### Scenario: Issues of other worklanes
- **GIVEN** v1.0.0 First production release also holds REBL-26 of Agenda and publication
- **WHEN** Brian opens the List of Fan accounts and push
- **THEN** v1.0.0's section shows no row for REBL-26

### Requirement: List ends with "No release" and this worklane's issues in no release
After the releases, a worklane's List SHALL show the section "No release" with the number of this worklane's issues in no release and those issues, while there is at least one.

#### Scenario: Two issues in no release
- **GIVEN** REBL-24 and REBL-27 of Fan accounts and push are in no release
- **WHEN** Brian opens its List
- **THEN** its last section is "No release" with "2 issues", REBL-24 and REBL-27

### Requirement: A List without releases and issues says why
While its repository's line holds no release and the worklane has no issue in no release, a worklane's List SHALL show "No releases planned for this worklane yet.", or "{repository} does not release, so its worklanes plan no releases." for a repository that does not release.

#### Scenario: Nothing planned
- **GIVEN** appboypov/rebl-admin has no releases and its worklane `main` holds no issues
- **WHEN** Brian opens the List of `main`
- **THEN** it shows "No releases planned for this worklane yet."

#### Scenario: A repository that does not release
- **GIVEN** appboypov/rebl-site is set to "This repository does not release" and its worklane `main` holds no issues
- **WHEN** Brian opens the List of `main`
- **THEN** it shows "appboypov/rebl-site does not release, so its worklanes plan no releases."
