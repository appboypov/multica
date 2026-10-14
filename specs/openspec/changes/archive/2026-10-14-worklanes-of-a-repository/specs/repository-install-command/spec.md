## ADDED Requirements

### Requirement: Multica finds a repository's install command
Add repository SHALL show Install for the picked repository once its checkout passes the check, filled in with the install command Multica found in the checkout's default branch from the first file of this list that it holds, and SHALL offer the command of every other file it holds; None SHALL be filled in when it holds none of them.

| File | Install command |
|---|---|
| `pnpm-lock.yaml` | `pnpm install` |
| `package-lock.json` | `npm ci` |
| `yarn.lock` | `yarn install` |
| `pubspec.yaml` | `flutter pub get` |
| `uv.lock` | `uv sync` |

#### Scenario: A pnpm repository
- **GIVEN** appboypov/rebl-admin's default branch holds `pnpm-lock.yaml` and its checkout passes the check
- **WHEN** Brian picks appboypov/rebl-admin in Add repository
- **THEN** Install shows `pnpm install`

#### Scenario: Two files
- **GIVEN** appboypov/fan-widgets' default branch holds `pubspec.yaml` and `uv.lock`
- **WHEN** Brian picks appboypov/fan-widgets in Add repository
- **THEN** Install shows `flutter pub get` and offers `uv sync`

#### Scenario: None of the files
- **GIVEN** appboypov/das-overview's default branch holds none of the five files
- **WHEN** Brian picks appboypov/das-overview in Add repository
- **THEN** Install shows None

### Requirement: Brian changes the install command or sets None
Install SHALL let Brian pick an offered command, type another command, or pick None, in Add repository and in Edit repository, and the repository SHALL keep what he saved. Edit repository SHALL show the repository's install command. A repository without an install command when this change arrives SHALL get the one Multica finds.

#### Scenario: Typed by hand
- **GIVEN** Brian picked appboypov/rebl-admin in Add repository
- **WHEN** he types `pnpm install --frozen-lockfile` as Install and adds the repository
- **THEN** Edit repository for appboypov/rebl-admin shows `pnpm install --frozen-lockfile`
- **AND** New worklane for appboypov/rebl-admin shows `pnpm install --frozen-lockfile`

#### Scenario: Set to None
- **WHEN** Brian picks None as Install in Edit repository for appboypov/rebl and saves
- **THEN** New worklane for appboypov/rebl says it installs nothing

#### Scenario: A repository from before
- **GIVEN** appboypov/rebl was added before repositories had an install command and its default branch holds `pnpm-lock.yaml`
- **WHEN** Brian opens Edit repository for appboypov/rebl
- **THEN** Install shows `pnpm install`

### Requirement: The brain sync gives a repository it adds the install command found
The brain sync SHALL give each repository it adds the install command Add repository would fill in. It MUST NOT change the install command of a repository already in Multica.

#### Scenario: A synced repository
- **GIVEN** the brain links appboypov/plannotator, whose default branch holds `package-lock.json`, to the Product MadSpec
- **WHEN** the brain sync adds appboypov/plannotator to MadSpec
- **THEN** Edit repository for appboypov/plannotator shows `npm ci`

#### Scenario: Set in Multica
- **GIVEN** Brian set appboypov/madspec's install command to None in Multica
- **WHEN** the brain sync runs
- **THEN** appboypov/madspec's install command is still None
