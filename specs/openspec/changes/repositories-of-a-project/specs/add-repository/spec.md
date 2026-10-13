## ADDED Requirements

### Requirement: Add repository lists the repositories the GitHub App can see
Repository on a project's page SHALL open Add repository, which SHALL list every repository the workspace's GitHub App installations can see, searchable by `owner/name`, each with its `owner/name`, Private or Public, and its default branch. A repository that is already in a project SHALL show that project's name and MUST NOT be pickable. Add repository MUST NOT offer a field to type a repository URL.

#### Scenario: The list
- **GIVEN** the GitHub App can see appboypov/rebl-admin, private with default branch main, and appboypov/rebl-site, public with default branch main
- **WHEN** Brian clicks Repository on the project Rebl's page
- **THEN** Add repository opens with the title "Add repository · Rebl"
- **AND** it lists appboypov/rebl-admin with "Private · main" and appboypov/rebl-site with "Public · main"
- **AND** it has no field for a repository URL

#### Scenario: Searching
- **GIVEN** Add repository lists appboypov/rebl-admin and appboypov/photoprank
- **WHEN** Brian types "rebl" in Search repositories
- **THEN** appboypov/rebl-admin shows and appboypov/photoprank does not

#### Scenario: No match
- **WHEN** Brian types "zzz" in Search repositories
- **THEN** the list says no repository matches "zzz"

#### Scenario: A repository in another project
- **GIVEN** appboypov/photoprank is a repository of the project Photo Prank
- **WHEN** Brian opens Add repository on Rebl
- **THEN** appboypov/photoprank shows "In Photo Prank"
- **AND** clicking it does not pick it

#### Scenario: A repository already in this project
- **GIVEN** appboypov/rebl is a repository of Rebl
- **WHEN** Brian opens Add repository on Rebl
- **THEN** appboypov/rebl shows "In Rebl" and cannot be picked

### Requirement: A missing repository is one click from the GitHub App installation
Add repository SHALL offer "Missing one? Give Multica access on GitHub", which SHALL open the GitHub App's installation page, where Brian grants access to more repositories.

#### Scenario: Giving access
- **GIVEN** the GitHub App cannot see appboypov/fan-widgets
- **WHEN** Brian clicks "Missing one? Give Multica access on GitHub"
- **THEN** the GitHub App's installation page opens
- **AND** after he grants access to appboypov/fan-widgets and opens Add repository again, it lists appboypov/fan-widgets

### Requirement: The checkout is a runtime and a checked path
Checkout SHALL ask for an online runtime and the path of the repository's normal checkout on it, filled in with `~/Repos/{name}` for the picked repository and changeable with Choose, the folder picker. Multica SHALL check on the runtime that the folder is a git checkout whose `origin` is the picked repository and which is on the repository's default branch, and SHALL show the passed check or each failed check under Checkout with how to fix it. An offline runtime MUST NOT be pickable.

#### Scenario: A valid checkout
- **GIVEN** `~/Repos/rebl-admin` on Brian's Mac is a git checkout of appboypov/rebl-admin on main
- **WHEN** Brian picks appboypov/rebl-admin
- **THEN** Checkout shows "Brian's Mac · online" and `~/Repos/rebl-admin`
- **AND** under it "Git checkout of appboypov/rebl-admin", "on main · origin matches"

#### Scenario: Another folder
- **GIVEN** Brian picked appboypov/rebl-admin
- **WHEN** he clicks Choose and picks `~/Code/rebl-admin`, a git checkout of appboypov/rebl-admin on main
- **THEN** Checkout shows `~/Code/rebl-admin` and its check passes

#### Scenario: Origin is another repository
- **GIVEN** `~/Repos/rebl-admin` is a git checkout whose `origin` is appboypov/rebl
- **WHEN** Brian picks appboypov/rebl-admin
- **THEN** Checkout says the folder's `origin` is appboypov/rebl, not appboypov/rebl-admin, and to pick its checkout of appboypov/rebl-admin

#### Scenario: Not on the default branch
- **GIVEN** `~/Repos/rebl-admin` is a git checkout of appboypov/rebl-admin on the branch feature/login
- **WHEN** Brian picks appboypov/rebl-admin
- **THEN** Checkout says the folder is on feature/login, not main, and to switch it to main

#### Scenario: Not a git checkout
- **GIVEN** `~/Code/notes` is a folder without git
- **WHEN** Brian picks appboypov/rebl-admin and chooses `~/Code/notes`
- **THEN** Checkout says the folder is not a git checkout

#### Scenario: An offline runtime
- **GIVEN** the runtime Office iMac is offline
- **WHEN** Brian opens the runtime list under Checkout
- **THEN** Office iMac shows as offline and cannot be picked

### Requirement: Clone it there clones into a folder that does not exist
When the checkout's folder does not exist, Checkout SHALL say so and offer "Clone it there", which SHALL clone the repository into that folder on the runtime, show that it is cloning, and then run the checkout's check. A failed clone SHALL show its error under Checkout with how to fix it.

#### Scenario: Cloning
- **GIVEN** no folder exists at `~/Repos/rebl-site` on Brian's Mac
- **WHEN** Brian picks appboypov/rebl-site
- **THEN** Checkout says there is no folder at `~/Repos/rebl-site` and offers Clone it there
- **AND** after he clicks Clone it there, it shows "Cloning appboypov/rebl-site" and then "Git checkout of appboypov/rebl-site", "on main · origin matches"
- **AND** `~/Repos/rebl-site` on Brian's Mac is a clone of appboypov/rebl-site on main

#### Scenario: The clone fails
- **GIVEN** no folder exists at `~/Repos/rebl-site` and Brian's Mac cannot reach GitHub
- **WHEN** Brian clicks Clone it there
- **THEN** Checkout shows the clone's error with how to fix it
- **AND** Add repository stays disabled

### Requirement: Releases shows the version policy Multica found
Releases SHALL offer, for the picked repository, the version file Multica found in its default branch, looking for `pubspec.yaml`, `Cargo.toml`, `pyproject.toml` and `package.json` and offering each it finds with the first in that order picked, then "Tags only" and "This repository does not release"; with none of these files it SHALL pick "Tags only". Every policy that releases SHALL show its tags as `v{version}` and, under Releases, the repository's last tag, the highest `v{version}` tag, or "No tags yet". "This repository does not release" SHALL say its worklanes plan no releases.

#### Scenario: One version file
- **GIVEN** appboypov/rebl-admin has `package.json` and the tags v0.4.1 and v0.4.2
- **WHEN** Brian picks appboypov/rebl-admin
- **THEN** Releases shows "Version in package.json · tags v{version}" and "Last tag v0.4.2"
- **AND** it offers "Tags only · tags v{version}" and "This repository does not release"

#### Scenario: Two version files
- **GIVEN** appboypov/fan-widgets has `pubspec.yaml` and `package.json`
- **WHEN** Brian picks appboypov/fan-widgets
- **THEN** Releases shows "Version in pubspec.yaml · tags v{version}"
- **AND** it offers "Version in package.json · tags v{version}", "Tags only · tags v{version}" and "This repository does not release"

#### Scenario: No version file
- **GIVEN** appboypov/rebl-site has none of the four files and the tag v2.1.0
- **WHEN** Brian picks appboypov/rebl-site
- **THEN** Releases shows "Tags only · tags v{version}" and "Last tag v2.1.0"

#### Scenario: No tags
- **GIVEN** appboypov/fan-widgets has no `v{version}` tag
- **WHEN** Brian picks appboypov/fan-widgets
- **THEN** Releases shows "No tags yet"

#### Scenario: Does not release
- **WHEN** Brian picks "This repository does not release"
- **THEN** Releases says its worklanes plan no releases

### Requirement: Add repository creates the repository with worklane main
Add repository SHALL stay disabled until a repository is picked and its checkout passes the check. Clicking it SHALL create the repository in the project with its checkout, its version policy and the worklane `main` on its checkout, Free, and SHALL show the project's page with the new repository's card open.

#### Scenario: Adding appboypov/rebl-admin
- **GIVEN** Brian picked appboypov/rebl-admin with the checked checkout `~/Repos/rebl-admin` on Brian's Mac and "Version in package.json"
- **WHEN** he clicks Add repository
- **THEN** Rebl's page shows the card appboypov/rebl-admin open, with default branch main and worklane `main` Free
- **AND** Add repository on another project shows appboypov/rebl-admin as "In Rebl"

#### Scenario: Nothing picked
- **WHEN** Brian opens Add repository and picks no repository
- **THEN** Add repository is disabled

#### Scenario: A failed check
- **GIVEN** `~/Repos/rebl-admin` is a git checkout of appboypov/rebl-admin on feature/login
- **WHEN** Brian picks appboypov/rebl-admin
- **THEN** Add repository is disabled
