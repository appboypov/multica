## ADDED Requirements

### Requirement: Link a pull request from the CLI
The `multica` CLI SHALL link a GitHub pull request to an issue by the pull request's URL for an agent in a run and for a member login. The link SHALL hold when Multica has not received that pull request from GitHub yet. A pull request of a repository no GitHub installation of the workspace covers MUST be refused with an error that names the repository, and nothing is linked.

#### Scenario: Joeri links his pull request
- **GIVEN** Joeri works WORK-343 in a run and opened `https://github.com/appboypov/rebl/pull/31` titled "Refuse malformed stored team members"
- **WHEN** Joeri links that URL to WORK-343 from the CLI
- **THEN** the command succeeds
- **AND** WORK-343's Pull requests section shows PR 31

#### Scenario: The webhook has not arrived yet
- **GIVEN** Joeri opened `https://github.com/appboypov/rebl/pull/31` a second ago and Multica has not received it from GitHub
- **WHEN** Joeri links that URL to WORK-343 from the CLI
- **THEN** the command succeeds
- **AND** WORK-343's Pull requests section shows PR 31

#### Scenario: A repository Multica cannot see
- **GIVEN** no GitHub installation of the workspace covers `github.com/acme/widget`
- **WHEN** an agent links `https://github.com/acme/widget/pull/4` to WORK-343 from the CLI
- **THEN** the command fails with an error that names `acme/widget`
- **AND** WORK-343 shows no new pull request

#### Scenario: A pull request Brian removed
- **GIVEN** Brian removed PR 31 from WORK-343 on the issue page
- **WHEN** Joeri links PR 31 to WORK-343 from the CLI
- **THEN** WORK-343 shows PR 31 again

### Requirement: One pull request shows on every issue it does work for
A pull request SHALL show on every issue it is linked to, and linking it to one issue MUST leave its links to other issues as they are.

#### Scenario: PR 26 holds the fixes of five tasks
- **GIVEN** Rebl PR 26 is linked to WORK-339
- **WHEN** an agent links PR 26 to WORK-340, WORK-341, WORK-342 and WORK-343
- **THEN** each of WORK-339 to WORK-343 shows PR 26

### Requirement: A run's instructions tell the agent to link its pull requests
The instructions of every run on an issue SHALL tell the agent to link, with the CLI command that links a pull request, each pull request it opens for the issue and each open pull request it pushes the issue's work to.

#### Scenario: The instructions of a run
- **GIVEN** Joeri is assigned WORK-343
- **WHEN** his run on WORK-343 starts
- **THEN** its instructions tell him to link each pull request he opens for WORK-343 and each open pull request he pushes WORK-343's work to
- **AND** they name the CLI command that links a pull request

### Requirement: Open pull requests that show on no issue are linked once
Each pull request that was open and showed on no issue before this change SHALL be linked once to the issue whose work it holds. A merged or closed pull request MUST NOT be linked by this pass, and a pull request for which no issue holds its work MUST stay unlinked.

#### Scenario: An open pull request named after an old key
- **GIVEN** Rebl PR 27 "Data-flow prototype Account and profile (REBL-136)" is open and shows on no issue
- **AND** a WORK issue holds the work of that data-flow prototype
- **WHEN** the one-time pass has run
- **THEN** that WORK issue shows PR 27

#### Scenario: A merged pull request stays as it is
- **GIVEN** Rebl PR 26 is merged and shows on no issue
- **WHEN** the one-time pass has run
- **THEN** PR 26 shows on no issue

#### Scenario: No issue holds the work
- **GIVEN** omp-messenger PR 19 is open and no issue in the workspace holds its work
- **WHEN** the one-time pass has run
- **THEN** PR 19 shows on no issue
