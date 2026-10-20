# multica-projects Specification

## Purpose

Fill the local Multica workspace with the brain's organisations, one project per Product and each Product's repositories, by hand with `madspec-sync/multica_projects.py`. Merge the repository projects UJ-1 retained only after their repositories are on the Product's project.

## Requirements

### Requirement: Each organisation that holds a Product is an organisation in Multica

The sync SHALL place every Organisation a Product links in Multica. It SHALL follow its identity mark when that organisation still exists, otherwise take over an organisation with the same name ignoring case, otherwise create one with the brain's name. An Organisation that no Product links SHALL NOT be created.

#### Scenario: The first run makes the organisations
- **GIVEN** the brain holds 48 Organisations, 14 of which a Product links
- **AND** Multica holds no organisation
- **WHEN** the sync runs
- **THEN** Multica holds those 14 organisations with their brain names

#### Scenario: An organisation made by hand is taken over
- **GIVEN** Brian made the organisation "rebl records" in Multica
- **AND** the brain's Product Rebl links Rebl Records
- **WHEN** the sync runs
- **THEN** that same organisation holds Rebl, without a second organisation

### Requirement: An organisation is created with its key

An organisation the sync creates SHALL have its Organisation's Linear key in lowercase, or the key in this table when it has no Linear key. An existing organisation's key SHALL stay unchanged. An Organisation with neither key SHALL be reported "has no key", its Products SHALL be reported with it, and none of them SHALL be placed.

| Organisation | Key |
| --- | --- |
| Marvelous K | `mk` |
| Mens & Maatwerk | `mnm` |
| InclusiTalent | `incl` |
| Buttler | `btlr` |
| Whitebird | `wb` |

#### Scenario: Keys on the first run
- **GIVEN** De App Specialist has Linear key DAS and Buttler has none
- **WHEN** the sync creates their organisations
- **THEN** De App Specialist has key `das` and Buttler has key `btlr`

#### Scenario: A key Brian changed stays
- **GIVEN** Brian changed Rebl Records' key to `rr` in Multica
- **WHEN** the sync runs
- **THEN** Rebl Records keeps `rr`

#### Scenario: An organisation without a key cannot be placed
- **GIVEN** the Product Fan app links an Organisation without a Linear key or an entry in the keys table
- **WHEN** the sync runs
- **THEN** it reports the Organisation and Fan app as having no organisation key
- **AND** it creates neither

### Requirement: Each Product is one project in its organisation

Each Product SHALL be one project, including a Product without repositories. The sync SHALL follow its mark when the marked project exists, otherwise take over an unclaimed project with exactly the Product's title, otherwise create one with that title and the Product's organisation. Checkout marks SHALL NOT participate in take-over. Other unmarked projects SHALL stay as they are. Each successful create or take-over SHALL record the identity before dependent writes.

The sync SHALL set the organisation of an existing project only when that project has none. An organisation Brian set SHALL stay. It SHALL NOT create a project for a Repository.

#### Scenario: A Product with three repositories
- **GIVEN** MadSpec of De App Specialist links madspec, plannotator and omp-anytype-inbox, each with one checkout
- **WHEN** the sync runs
- **THEN** one project MadSpec belongs to De App Specialist and holds those three repositories

#### Scenario: A Product without repositories
- **GIVEN** DAS HQ of De App Specialist links no Repository
- **WHEN** the sync runs
- **THEN** the project DAS HQ belongs to De App Specialist without a repository

#### Scenario: A new Product
- **GIVEN** a synced workspace
- **WHEN** a Product is added to the brain and the sync runs
- **THEN** one new project with that Product's name exists under its organisation
- **AND** no other project changes

#### Scenario: An exact-title hand-made project is taken over
- **GIVEN** Brian made MadSpec in Multica without an organisation
- **WHEN** the sync runs
- **THEN** that same project belongs to De App Specialist and gets MadSpec's repositories
- **AND** no second MadSpec project is made

#### Scenario: A lost mark file after a merge
- **GIVEN** Rebl holds the issues and repository appboypov/rebl
- **WHEN** the mark file is lost and the sync runs
- **THEN** that same Rebl project is taken over and recorded
- **AND** no project is created, moved or deleted

#### Scenario: Brian moved a synced project
- **GIVEN** Brian moved Rebl from Rebl Records to De App Specialist
- **AND** the brain's Product Rebl still links Rebl Records
- **WHEN** the sync runs
- **THEN** Rebl stays under De App Specialist

#### Scenario: A synced project without an organisation
- **GIVEN** the synced project Rebl has no organisation
- **WHEN** the sync runs
- **THEN** Rebl belongs to its Product's organisation Rebl Records

### Requirement: Each repository of a Product is a repository of its project

The sync SHALL read `GET /api/repositories` and `GET /api/repositories/available`. For each brain Repository that links one placed Product, it SHALL add the repository to that Product's project if it is absent from Multica, using Add repository's runtime checkout check. The server creates worklane `main` on the checkout, Free, and removes the obsolete checkout resource. The sync SHALL never create a checkout resource or a repository project.

Checkouts SHALL be found under `~/Repos` at any depth by their GitHub `origin`, matching the GitHub key ignoring case and excluding `~/Repos/Archive`. Lookup SHALL continue inside git folders without a GitHub origin. A missing checkout or several checkouts SHALL prevent addition.

The check SHALL use the local omp runtime, `clone: false`, and the repository name and default branch returned by the GitHub App's available-repositories endpoint. It SHALL check existence, git work tree, origin and current branch. The sync SHALL poll until completion or its bounded wait ends. A passed check SHALL supply the `checkout_check_id`; the version policy SHALL be its first found version file in the check's ordering, or `tags` when no version file was found.

#### Scenario: MadSpec's repositories
- **GIVEN** MadSpec links appboypov/madspec, appboypov/plannotator and appboypov/omp-anytype-inbox, each with one checkout on its default branch
- **WHEN** the sync runs
- **THEN** MadSpec holds those three repositories with worklane `main` Free
- **AND** none retains the sync's obsolete local folder resource

#### Scenario: The version policy
- **GIVEN** appboypov/rebl has `pubspec.yaml` and `package.json` on its default branch
- **WHEN** the sync adds it
- **THEN** its version policy is `pubspec.yaml`

#### Scenario: Archive and mixed-case keys
- **GIVEN** a Repository's mixed-case key matches a live checkout and a copy under `~/Repos/Archive`
- **WHEN** the sync runs
- **THEN** it checks and adds the live checkout only

### Requirement: A repository the sync cannot add is reported and left alone

On every run the sync SHALL report each repository the GitHub App cannot see, with no checkout or several, or whose checkout fails a check. An existing repository SHALL never be moved or updated; when it belongs to another project, the sync SHALL print `in {project}`. Once addition is possible, the first subsequent run SHALL add it.

#### Scenario: Not on the default branch
- **GIVEN** appboypov/rebl's checkout is on feature/login rather than main
- **WHEN** the sync runs
- **THEN** it prints the wrong-branch failure and adds no repository
- **AND** a subsequent run after switching to main adds it

#### Scenario: In another project Brian chose
- **GIVEN** Brian added appboypov/rebl-admin to Rebl but the brain links it to Rebl Admin
- **WHEN** the sync runs
- **THEN** the repository stays in Rebl and no issue moves out of Rebl
- **AND** the sync prints that appboypov/rebl-admin is in Rebl

### Requirement: What Brian set on a repository stays

The sync MUST NOT change an existing repository's checkout, version policy or install command. It SHALL skip its checkout check as well as its update. While its brain Repository remains linked, removing the repository from Multica SHALL cause it to be added again on the next eligible run. A synced Repository deleted from the brain SHALL stay in Multica and SHALL be printed on every run.

#### Scenario: An edited checkout and policy
- **GIVEN** Brian set appboypov/rebl's checkout to `~/Code/rebl` and policy to `none`
- **WHEN** the sync runs
- **THEN** both values stay unchanged

#### Scenario: Removed in Multica, linked in the brain
- **GIVEN** Brian removed appboypov/plannotator from MadSpec but the brain still links it
- **WHEN** the sync runs
- **THEN** it is added again with worklane `main`

#### Scenario: Deleted from the brain
- **GIVEN** MadSpec holds synced appboypov/omp-anytype-inbox
- **WHEN** its Repository object is deleted from the brain and the sync runs
- **THEN** it stays in Multica
- **AND** the sync prints that it has no Repository in the brain

### Requirement: Kept repository projects merge into their Product's project

The `repository_projects` marks SHALL identify the projects UJ-1 kept. Once each marked repository is on its Product's project, the sync SHALL move the retained project's issues, including closed issues, into that Product's project. A repository not added there, including one Brian put in another project, SHALL keep its source project and issues. If several marks point to one source, all must identify repositories placed on the same Product's project. No marked Product project SHALL be deleted.

The sync SHALL read candidate issue IDs before entity writes. It SHALL use `POST /api/issues/batch-update` with pages of at most 100 and `{issue_ids, updates: {project_id}}`. Every page SHALL return the full requested `updated` count. Only after all pages succeed and a fresh `GET /api/issues?project_id={id}` returns `total` zero SHALL it delete the emptied project and drop its `repository_projects` entries. A failed or partial move, failed fresh read or nonzero fresh total SHALL keep the project and make the run fail. Failure to add or mark a repository SHALL skip its dependent merge.

A marked source project that no longer exists SHALL have its entry removed. A Repository with no Product or gone from the brain SHALL keep its source project and issues. The sync SHALL neither reconstruct source identity from checkout labels nor delete any other project.

#### Scenario: MadSpec takes in its repositories' issues
- **GIVEN** UJ-1 kept madspec, plannotator and omp-anytype-inbox with 10, 2 and 1 issues
- **WHEN** the sync adds their repositories to MadSpec
- **THEN** MadSpec holds those 13 issues
- **AND** the three emptied projects and their retained-project marks are gone

#### Scenario: A repository cannot be added
- **GIVEN** the GitHub App cannot see Supermarty-B-V/sm-apify
- **WHEN** the sync runs
- **THEN** the reason is printed and the retained sm-apify project stays with its issues

#### Scenario: A partial move keeps the repository project
- **GIVEN** a repository project has 101 issues
- **WHEN** one page cannot move every requested issue
- **THEN** the source stays and the run exits nonzero
- **AND** the next run plans from its remaining issues

#### Scenario: An issue arrives before deletion
- **GIVEN** all pre-read issues moved successfully
- **WHEN** the fresh read still counts an issue in the source
- **THEN** the project and its mark remain and the incomplete merge is reported


### Requirement: Ambiguous parent links are reported and left alone

Products with multiple Organisation links and Repositories with multiple Product links SHALL be reported and left alone. Missing or unplaced parents SHALL be reported without guessing a destination. Retained projects whose marks do not identify repositories on one placed Product SHALL be reported and kept. An existing marked Product project SHALL NOT be deleted as a repository project.

#### Scenario: A Repository links two Products
- **GIVEN** a Repository links both MadSpec and DAS HQ
- **WHEN** the sync runs
- **THEN** it reports the multiple Product links and leaves that Repository's project and issues alone

### Requirement: What leaves the brain stays in Multica

A marked Product or Organisation gone from the brain SHALL be printed on every run and left as it is with its projects and issues. Identity SHALL be followed by brain ID rather than name when the mark exists. The sync SHALL never rename a project or organisation, change an existing organisation key, or write to the brain. It SHALL NOT write project status, dates, description, icon, lead or priority.

#### Scenario: A Product deleted from the brain
- **GIVEN** the synced project Photo Prank has issues
- **WHEN** its Product is deleted from the brain and the sync runs
- **THEN** Photo Prank stays with its issues under its organisation
- **AND** the sync prints that Photo Prank has no Product in the brain

#### Scenario: An Organisation deleted from the brain
- **GIVEN** the synced organisation Profa
- **WHEN** its Organisation is deleted from the brain and the sync runs
- **THEN** Profa stays with its projects
- **AND** the sync prints that Profa has no Organisation in the brain

#### Scenario: A Product renamed in the brain
- **GIVEN** the synced project Float Note
- **WHEN** its Product is renamed Floatnote and the sync runs
- **THEN** the same project keeps its title Float Note
- **AND** the brain is unchanged by the sync

### Requirement: The sync keeps an atomic identity mark

`~/.local/state/madspec/multica/brain-sync.json` SHALL map Organisation, Product and added Repository brain IDs to Multica IDs, separated by server URL and workspace UUID. Server URLs SHALL be normalized by dropping a trailing slash. Its JSON shape is `{server_url: {workspace_id: {organisations: {brain_id: multica_id}, products: {brain_id: multica_id}, repositories: {brain_id: multica_id}, repository_projects: {brain_id: retained_project_id}}}}`.

After every successful create or take-over, the sync SHALL write the mark through a flushed/fsynced temporary file followed by an atomic rename. Mark writes SHALL preserve other server/workspace namespaces even when profiles run concurrently. Invalid or unreadable marks SHALL fail the read before server writes. A missing mark file SHALL be rebuilt through the take-over rules; a missing marked Multica row SHALL use those same rules. Failure to save an identity SHALL skip its dependent writes.

If both the mark file is lost and Brian renamed a Product project, exact-title take-over cannot recover that identity and a project may be created again. The sync does not infer identity from repository marks for Product projects.

The `repository_projects` map SHALL preserve UJ-1's retained project identity until that project merges successfully or no longer exists. New retained-project marks SHALL NOT be created from checkout resources. Each successful repository addition SHALL be recorded under `repositories`, allowing a Repository object deleted from the brain to be reported without deleting the Multica repository.

### Requirement: A failed read writes nothing

The sync SHALL read Organisations, Products, Repositories and their links, checkouts, Multica organisations, projects, repositories, GitHub App-visible repositories, candidate issue IDs and its marks before entity writes. Checkout-check requests are read operations on the runtime, not repository additions. A failed read or a brain answer with no Product object SHALL print the failure, exit nonzero and make no organisation, project, repository or issue change. An empty Repository list SHALL be valid. A checkout failure SHALL report that repository and leave it alone. A failed entity write SHALL be printed, skip dependent work, continue independent work and make the run exit nonzero.

#### Scenario: A source is unreachable
- **GIVEN** a synced workspace
- **WHEN** a brain, GitHub or Multica read fails
- **THEN** no project, organisation, repository or issue changes
- **AND** the sync prints the failure and exits nonzero

#### Scenario: The brain answers with no Product objects
- **GIVEN** a synced workspace
- **WHEN** the Product search returns no objects
- **THEN** no write is sent to Multica and the sync exits nonzero

### Requirement: A sync writes only what differs

The sync SHALL print one line per write or identity mark and per reported object, and SHALL send a server write only for a difference. A second unchanged run SHALL send no write.

#### Scenario: A second run changes nothing
- **GIVEN** a successful sync run
- **WHEN** the sync runs again without source changes
- **THEN** it sends no write to Multica and creates no duplicate organisation, project or repository

### Requirement: The sync runs by hand with one profile lock

The tool SHALL run only when started by hand from the Multica checkout. It SHALL load the brain and checkout readers from MadSpec's `team_state.py` in `~/Repos/Ventures/madspec`, and the workspace transport from this folder's `multica_api.py`; it SHALL select the built-in local omp runtime for checkout checks itself, excluding persona runtime profiles. Credentials SHALL come from `~/.multica/profiles/local/config.json`. A missing dependency SHALL fail without writes.

Runs of one profile SHALL hold `projects.lock` next to that profile's config across reads, plan and writes. A waiting run SHALL plan from the completed prior run's server data and mark file. A short `brain-sync.lock` beside the global mark file SHALL serialize its read/modify/rename operations across profiles.

#### Scenario: Two runs at once
- **GIVEN** a Product with one eligible Repository and no organisation or project in Multica
- **WHEN** two sync runs start at the same time
- **THEN** both exit successfully
- **AND** Multica holds one organisation, one project for that Product and one repository

### Requirement: A repository added by the sync keeps the install command found

When the sync adds a repository, it SHALL send `install_command` from the first
command in its passed checkout check's `install_commands`, or `''` (None) when
the check found none. It SHALL NOT update the install command of a repository
already in Multica.

#### Scenario: A synced repository uses the command found
- **GIVEN** appboypov/plannotator is added to MadSpec and its passed checkout check found `npm ci`
- **WHEN** the sync adds that repository
- **THEN** it sends `install_command: "npm ci"`

#### Scenario: A saved None stays unchanged
- **GIVEN** appboypov/madspec already exists in Multica with Install set to None
- **WHEN** the sync runs
- **THEN** that repository's install command stays None

