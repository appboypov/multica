# brain-project-sync Specification

## Purpose

The brain sync fills Multica's organisations and projects from the brain's Organisation and Product objects in one run: one project per Product in its organisation, the repository project of a Product with one repository merged into it, the repository projects of a Product with several repositories left as they are and printed, and nothing written to the brain.

## Requirements

### Requirement: Each organisation that holds a Product is an organisation in Multica
The brain sync SHALL create in Multica each Organisation object of the brain that a Product object links, with the Organisation's name. It SHALL take over an organisation made in Multica whose name equals that name, ignoring case, instead of creating a second one. It MUST NOT create an organisation for an Organisation object that no Product links.

#### Scenario: The first run makes the organisations
- **GIVEN** the brain holds 48 Organisation objects, 14 of which a Product links
- **AND** the workspace holds no organisation
- **WHEN** the sync runs
- **THEN** the workspace holds those 14 organisations, each with its Organisation's name

#### Scenario: An organisation made by hand is taken over
- **GIVEN** Brian made the organisation "rebl records" in Multica
- **AND** the brain's Product Rebl links the Organisation Rebl Records
- **WHEN** the sync runs
- **THEN** the workspace holds one organisation with that name, and the project Rebl belongs to it

### Requirement: The sync gives each organisation it creates its key
The brain sync SHALL give each organisation it creates the Linear key of its Organisation object in lowercase, or for an Organisation without a Linear key the key in this table. It MUST NOT change the key of an organisation that exists in Multica.

| Organisation | Key |
|---|---|
| Marvelous K | `mk` |
| Mens & Maatwerk | `mnm` |
| InclusiTalent | `incl` |
| Buttler | `btlr` |
| Whitebird | `wb` |

#### Scenario: Keys on the first run
- **GIVEN** the Organisation De App Specialist has the Linear key DAS and Buttler has none
- **WHEN** the sync creates both organisations
- **THEN** De App Specialist has the key `das` and Buttler the key `btlr`

#### Scenario: A key Brian changed
- **GIVEN** Brian changed Rebl Records' key from `rebl` to `rr` in Multica
- **WHEN** the sync runs
- **THEN** Rebl Records keeps the key `rr`

### Requirement: Each Product is one project in its organisation
The brain sync SHALL keep one project per Product object of the brain, titled with the Product's name and, when the sync creates or takes over the project, in the organisation of the Product's Organisation. It SHALL take over a project without the sync's mark whose title equals the Product's name. The project of a Product that links one Repository SHALL hold that repository's checkout, as its repository project holds it today. The project of a Product that links several Repositories MUST NOT hold a checkout, because a project holds one checkout per runtime. The sync MUST NOT create a project for a repository.

#### Scenario: A Product with one repository
- **GIVEN** the Product Rebl of Rebl Records links only the repository appboypov/rebl, with one checkout
- **WHEN** the sync runs
- **THEN** the project Rebl shows under Rebl Records on Projects
- **AND** it holds the checkout of appboypov/rebl

#### Scenario: A Product with three repositories
- **GIVEN** the Product MadSpec of De App Specialist links the repositories appboypov/madspec, appboypov/plannotator and appboypov/omp-anytype-inbox, each with one checkout
- **WHEN** the sync runs
- **THEN** the project MadSpec shows under De App Specialist on Projects
- **AND** it holds no checkout

#### Scenario: A Product without repositories
- **GIVEN** the Product DAS HQ of De App Specialist links no repository
- **WHEN** the sync runs
- **THEN** the project DAS HQ shows under De App Specialist without a checkout

#### Scenario: A new Product
- **GIVEN** a synced workspace
- **WHEN** a Product is added to the brain and the sync runs
- **THEN** one new project with that Product's name shows under its organisation
- **AND** no other project changes

### Requirement: The repository project of a Product with one repository merges into its project
When a Product links one Repository, the brain sync SHALL move every issue of the project it made for that repository into the Product's project, and SHALL then delete the emptied repository project. It MUST NOT delete any other project.

#### Scenario: Rebl takes in its repository's issues
- **GIVEN** the repository project rebl holds 6 issues, and appboypov/rebl is the only repository of the Product Rebl
- **WHEN** the sync runs
- **THEN** the project Rebl holds those 6 issues and the checkout of appboypov/rebl
- **AND** the project rebl no longer exists

#### Scenario: A repository without a Product
- **GIVEN** the repository appboypov/das-duckmd-brain links no Product and its project das-duckmd-brain holds 8 issues
- **WHEN** the sync runs
- **THEN** it prints that appboypov/das-duckmd-brain has no Product
- **AND** das-duckmd-brain stays with its 8 issues and its checkout, under "No organisation"

### Requirement: The repository projects of a Product with several repositories stay
When a Product links several Repositories, the brain sync SHALL leave each project it made for one of them as it is, with its issues and its checkout, and SHALL print on every run that the project stays for that Product.

#### Scenario: MadSpec's repository projects stay
- **GIVEN** the repository projects madspec, plannotator and omp-anytype-inbox hold 10, 2 and 1 issues, and their repositories link the Product MadSpec
- **WHEN** the sync runs
- **THEN** the three projects stay with their issues and their checkouts, under "No organisation"
- **AND** the project MadSpec holds none of those issues
- **AND** the sync prints for each of the three that it stays for MadSpec, which has 3 repositories

### Requirement: An organisation set in Multica stays
The brain sync SHALL set a project's organisation only when it creates or takes over the project or when the project has no organisation, and MUST NOT change an organisation the project already has.

#### Scenario: Brian moved a synced project
- **GIVEN** Brian moved the synced project Rebl from Rebl Records to De App Specialist on its page
- **AND** the brain's Product Rebl still links Rebl Records
- **WHEN** the sync runs
- **THEN** Rebl stays under De App Specialist

#### Scenario: A synced project without an organisation
- **GIVEN** the synced project Rebl has no organisation and its Product links Rebl Records
- **WHEN** the sync runs
- **THEN** Rebl shows under Rebl Records

### Requirement: What leaves the brain stays in Multica
When a Product or Organisation object is deleted from the brain, the brain sync SHALL leave its project or organisation in Multica as it is and SHALL print it on every run.

#### Scenario: A Product deleted from the brain
- **GIVEN** the synced project Photo Prank with its issues
- **WHEN** its Product is deleted from the brain and the sync runs
- **THEN** Photo Prank stays with its issues under its organisation
- **AND** the sync prints that Photo Prank has no Product in the brain

#### Scenario: An Organisation deleted from the brain
- **GIVEN** the synced organisation Profa
- **WHEN** its Organisation object is deleted from the brain and the sync runs
- **THEN** Profa stays with its projects
- **AND** the sync prints that Profa has no Organisation in the brain

### Requirement: The sync never renames and never writes to the brain
The brain sync MUST NOT rename an organisation or a project and MUST NOT write to the brain.

#### Scenario: A Product renamed in the brain
- **GIVEN** the synced project Float Note
- **WHEN** its Product is renamed Floatnote in the brain and the sync runs
- **THEN** the project keeps the title Float Note
- **AND** the Product in the brain is unchanged by the run
