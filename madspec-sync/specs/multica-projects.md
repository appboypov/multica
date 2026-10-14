# multica-projects Specification

## Purpose

Fill the local Multica workspace with the brain's organisations and one project per Product, merging the repository project of each Product with one Repository and preserving the repository projects of Products with several, by hand with `madspec-sync/multica_projects.py`.

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

Each Product SHALL be one project, including a Product without repositories. The sync SHALL follow its mark when the marked project exists, otherwise take over a project with exactly the Product's title whose only checkout mark, if any, is that of the Product's only Repository, otherwise create a project with that title and the Product's organisation. Other unmarked projects SHALL stay as they are. Each successful create or take-over SHALL record the identity before dependent writes.

The sync SHALL set the organisation of an existing project only when that project has none. An organisation Brian set SHALL stay. It SHALL NOT create a project for a Repository.

#### Scenario: A Product with three repositories
- **GIVEN** MadSpec of De App Specialist links madspec, plannotator and omp-anytype-inbox, each with one checkout
- **WHEN** the sync runs
- **THEN** one project MadSpec belongs to De App Specialist and holds no checkout

#### Scenario: A Product without repositories
- **GIVEN** DAS HQ of De App Specialist links no Repository
- **WHEN** the sync runs
- **THEN** the project DAS HQ belongs to De App Specialist without a checkout

#### Scenario: A new Product
- **GIVEN** a synced workspace
- **WHEN** a Product is added to the brain and the sync runs
- **THEN** one new project with that Product's name exists under its organisation
- **AND** no other project changes

#### Scenario: An exact-title hand-made project is taken over
- **GIVEN** Brian made MadSpec in Multica without a checkout mark or organisation
- **WHEN** the sync runs
- **THEN** that same project belongs to De App Specialist without a checkout when MadSpec has several Repositories
- **AND** no second MadSpec project is made

#### Scenario: A lost mark file after a merge
- **GIVEN** Rebl holds the issues and checkout of its only Repository appboypov/rebl
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

### Requirement: A Product with one Repository holds its counting checkout

Counting SHALL control checkout placement only. A Repository counts when GitHub knows it and has not archived it, it has exactly one checkout outside `~/Repos/Archive`, and no other counting Repository shares its repository name ignoring case. The sync SHALL find checkouts under `~/Repos` at any depth by their GitHub `origin` remote, matching the GitHub key without regard to case; lookup SHALL continue inside git folders without a GitHub origin.

A Product's Repository count SHALL be the number of brain Repository objects that link it, including archived Repositories, Repositories without checkouts and distinct objects sharing a GitHub key. Only a Product with exactly one Repository SHALL get checkout writes. A Product with several Repositories SHALL get no checkout writes; its repository projects and their checkouts SHALL remain unchanged.

The only Repository of a placed Product, when it counts, SHALL have one marked `local_directory` resource on that Product's project, with:

- `local_path`: its checkout;
- `daemon_id`: the local omp runtime's daemon;
- `execution_mode`: `worktree`;
- `label`: `madspec:<owner/name>`, the lowercase GitHub key.

An existing marked resource on the Product's project SHALL be updated in place when these values differ. The sync SHALL NOT create a `github_repo` resource. Resource marks in the label column or resource ref SHALL identify the same Repository.

A Repository with no checkout, multiple checkouts, an unknown GitHub key, a shared name or an archived GitHub repository SHALL be reported with that reason and get no checkout resource placed on the Product's project. Its eligibility for an issue merge SHALL be independent of whether it counts, but its Product MUST have exactly one Repository.

#### Scenario: A checkout inside a git folder without origin
- **GIVEN** `~/Repos/Clients/paps` has no origin, and its nested whitebird-brain checkout has origin appboypov/whitebird-brain
- **WHEN** the sync looks up checkouts
- **THEN** the nested whitebird-brain checkout counts

#### Scenario: Archive and mixed-case keys
- **GIVEN** a Repository's mixed-case GitHub key matches one live checkout and a copy under `~/Repos/Archive`
- **WHEN** the sync runs
- **THEN** its Product project has one checkout resource on the live path with a lowercase mark

#### Scenario: A changed checkout is updated in place
- **GIVEN** a Product project's marked resource has a different path, daemon or execution mode
- **WHEN** the sync runs
- **THEN** the same resource has the counting Repository's path, local omp daemon and worktree mode
- **AND** the project's identity and issues stay unchanged

### Requirement: The repository project of a Product with one Repository merges into its project

A project carrying a checkout mark for the only Repository of a placed Product, and that is not that Product's project, SHALL merge into that Product's project. This SHALL apply whether or not that single Repository counts, including when its checkout is missing or ambiguous, its name is shared, GitHub does not know it, or GitHub archived it.

The sync SHALL read every candidate's issue IDs before any write, including closed issues. It SHALL move them through `POST /api/issues/batch-update` in pages of at most 100 with `{issue_ids, updates: {project_id}}`. Every page SHALL return the full requested `updated` count. Only after all pages succeed and a fresh `GET /api/issues?project_id={id}` returns `total` zero SHALL it delete the emptied project. A failed or partial move, failed fresh read or nonzero fresh total SHALL keep the repository project and make the run fail. A failed checkout write SHALL keep its source project. Deleting the project removes its resources.

A Repository without a Product, and a marked project whose Repository is gone from the brain, SHALL stay with its project, resources and issues and be printed on every run. No other project SHALL be deleted.

#### Scenario: Rebl takes in its repository's issues
- **GIVEN** the repository project rebl holds 6 issues and appboypov/rebl is Rebl's only Repository
- **WHEN** the sync runs
- **THEN** Rebl holds those 6 issues and the counting checkout of appboypov/rebl
- **AND** the emptied repository project rebl no longer exists

#### Scenario: A repository without a Product
- **GIVEN** das-duckmd-brain links no Product and its project holds 8 issues
- **WHEN** the sync runs
- **THEN** it prints that appboypov/das-duckmd-brain has no Product
- **AND** its project stays with those 8 issues and its checkout

#### Scenario: A linked Repository has no checkout
- **GIVEN** appboypov/rebl is Rebl's only Repository, has no live checkout and its repository project has 2 issues
- **WHEN** the sync runs
- **THEN** it reports the missing checkout and adds no checkout to Rebl
- **AND** its 2 issues move into Rebl and the emptied repository project is deleted

#### Scenario: A partial move keeps the repository project
- **GIVEN** a repository project has 101 issues
- **WHEN** one page cannot move every requested issue
- **THEN** that repository project is kept and the run exits nonzero
- **AND** the next run plans from its remaining issues

#### Scenario: An issue arrives before deletion
- **GIVEN** all pre-read issues of a repository project moved successfully
- **WHEN** a fresh read still counts an issue in that project
- **THEN** the sync keeps the project and reports the incomplete merge

### Requirement: Repository projects of a Product with several Repositories stay

For a placed Product with several brain Repository objects, the sync SHALL leave each marked repository project unchanged, with its issues and checkout. On every run it SHALL print "{project} stays for {Product}, which has {N} repositories". It SHALL atomically record each Repository's brain ID to that project's Multica ID under the scoped `repository_projects` identity map. An unchanged identity SHALL NOT be rewritten. A Repository without a repository project SHALL get neither a project nor a checkout resource, and when it has a counting checkout the sync SHALL print "{repository}: gets no checkout, because {Product} has {N} repositories" on every run.

#### Scenario: MadSpec's repository projects stay
- **GIVEN** MadSpec links madspec, plannotator and omp-anytype-inbox, whose repository projects hold 10, 2 and 1 issues
- **WHEN** the sync runs
- **THEN** each repository project keeps its issues and checkout
- **AND** MadSpec gets none of those issues or checkouts
- **AND** the three retained projects are printed and recorded by their Repository brain IDs

#### Scenario: A Repository without a repository project is printed
- **GIVEN** MadSpec links madspec, whose repository project holds its checkout, and plannotator with one checkout and no project
- **WHEN** the sync runs
- **THEN** it prints that appboypov/plannotator gets no checkout, because MadSpec has 2 repositories

#### Scenario: A Repository without a checkout still prevents a merge
- **GIVEN** MadSpec links madspec with one checkout and plannotator without a checkout
- **WHEN** the sync runs
- **THEN** MadSpec gets no checkout
- **AND** the repository project madspec stays with its issues and checkout and is printed and recorded

### Requirement: Ambiguous parent links are reported and left alone

Products with multiple Organisation links and Repositories with multiple Product links SHALL be reported and left alone. Missing or unplaced parents SHALL be reported without guessing a destination. A project with checkout marks that do not identify one placed Product SHALL be reported and kept. An existing marked Product project SHALL NOT be deleted as a repository project.

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

`~/.local/state/madspec/multica/brain-sync.json` SHALL map Organisation and Product brain IDs to Multica IDs, and retained Repository brain IDs to their repository project IDs, separated by server URL and workspace UUID. Server URLs SHALL be normalized by dropping a trailing slash. Its JSON shape is `{server_url: {workspace_id: {organisations: {brain_id: multica_id}, products: {brain_id: multica_id}, repository_projects: {brain_id: multica_id}}}}`.

After every successful create or take-over, the sync SHALL write the mark through a flushed/fsynced temporary file followed by an atomic rename. Mark writes SHALL preserve other server/workspace namespaces even when profiles run concurrently. Invalid or unreadable marks SHALL fail the read before server writes. A missing mark file SHALL be rebuilt through the take-over rules; a missing marked Multica row SHALL use those same rules. Failure to save an identity SHALL skip its dependent writes.

If both the mark file is lost and Brian renamed a Product project, exact-title take-over cannot recover that identity and a project may be created again. The sync does not infer identity from repository marks for Product projects.

The `repository_projects` map SHALL preserve the identity of retained repository projects for UJ-2, which removes checkout resources and their labels when it gives Product projects several repositories. As long as the checkout marks exist, retained entries SHALL be rebuilt from them when the mark file is missing.

### Requirement: A failed read writes nothing

The sync SHALL read Organisations, Products, Repositories and their links, checkouts, GitHub archive states, Multica organisations, projects and resources, every merge candidate's issue IDs, and its mark before writing. A failed read or a brain answer with no Product object SHALL print the failure, exit nonzero and send no write to Multica. An empty Repository list SHALL be valid for Products without repositories. GitHub NOT_FOUND SHALL report that Repository as unknown without failing unrelated reads. A failed write SHALL be printed, SHALL skip dependent work, SHALL NOT stop independent work, and SHALL make the run exit nonzero.

#### Scenario: A source is unreachable
- **GIVEN** a synced workspace
- **WHEN** a brain, GitHub or Multica read fails
- **THEN** no project, organisation, resource or issue changes
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
- **THEN** it sends no write to Multica and creates no duplicate organisation, project or resource

### Requirement: The sync runs by hand with one profile lock

The tool SHALL run only when started by hand from the Multica checkout. It SHALL load the brain and checkout readers from MadSpec's `team_state.py` in `~/Repos/Ventures/madspec`, and the workspace transport and local omp runtime selection from this folder's `multica_sync.py`. Credentials SHALL come from `~/.multica/profiles/local/config.json`. A missing dependency SHALL fail without writes.

Runs of one profile SHALL hold `projects.lock` next to that profile's config across reads, plan and writes. A waiting run SHALL plan from the completed prior run's server data and mark file. A short `brain-sync.lock` beside the global mark file SHALL serialize its read/modify/rename operations across profiles.

#### Scenario: Two runs at once
- **GIVEN** a Product with one counting Repository and no organisation or project in Multica
- **WHEN** two sync runs start at the same time
- **THEN** both exit successfully
- **AND** Multica holds one organisation, one project for that Product and one checkout resource

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

