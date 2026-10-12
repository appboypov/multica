# multica-projects Specification

## Purpose

Keep local Multica projects in line with the brain's Repository objects and their checkouts in worktree mode, by hand, with `madspec-sync/multica_projects.py`.

## Requirements

### Requirement: Every Repository object is a Multica project on its checkout

Every repository that counts SHALL be one project in the local Multica workspace. A repository counts when it has a Repository object in the brain, is not archived on GitHub and has exactly one checkout. Its project's title SHALL be the name part of the Repository's **GitHub key**. The project SHALL have one `local_directory` resource whose ref holds:

- `local_path`: the checkout;
- `daemon_id`: the daemon of the local omp runtime;
- `execution_mode`: `worktree`;
- `label`: `madspec:<owner/name>`, the lowercase GitHub key, which marks the project as the sync's own.

A project the sync creates SHALL start with status `in_progress` and no description.

#### Scenario: The first run makes the projects

- **GIVEN** the 24 Repository objects of today plus `appboypov/ivga-brain`, `appboypov/whitebird-brain`, `appboypov/floatnote`, `appboypov/multica` and `appboypov/turbo-herdr`
- **AND** `appboypov/work` has no checkout on this Mac
- **AND** the workspace holds no projects
- **WHEN** `multica_projects.py` runs
- **THEN** the workspace holds 28 projects, each `in_progress`, each with exactly one `local_directory` resource in `worktree` mode
- **AND** it holds no project named `work`

#### Scenario: A project carries its repository's name and checkout

- **GIVEN** the Repository object with GitHub key `Supermarty-B-V/sm-design` and its one checkout `~/Repos/Clients/sm/sm-design`
- **WHEN** the sync runs
- **THEN** the project `sm-design` has a `local_directory` resource with that checkout as `local_path`, the local omp daemon as `daemon_id`, `execution_mode` `worktree` and label `madspec:supermarty-b-v/sm-design`
- **AND** the project has no `github_repo` resource

#### Scenario: A new Repository object becomes a project

- **GIVEN** a synced workspace
- **WHEN** a Repository object is added for a repository with one checkout and the sync runs
- **THEN** one new project with that repository's name exists
- **AND** no other project changed

### Requirement: A repository's checkout is found by its origin remote

The sync SHALL find a repository's checkouts under `~/Repos`, at any depth, by the GitHub name of their `origin` remote, matching the Repository's **GitHub key** without regard to case. The search SHALL go on inside a git folder whose `origin` is not on GitHub or that has no `origin`. A checkout under `~/Repos/Archive` SHALL NOT count.

#### Scenario: A checkout inside a git folder without origin

- **GIVEN** `~/Repos/Clients/paps` is a git folder without an `origin` remote
- **AND** `~/Repos/Clients/paps/whitebird-brain` is a checkout of `appboypov/whitebird-brain`
- **WHEN** the checkouts are looked up
- **THEN** `appboypov/whitebird-brain` has the checkout `~/Repos/Clients/paps/whitebird-brain`

#### Scenario: An archived copy does not count

- **GIVEN** `appboypov/floatnote` has checkouts at `~/Repos/Apps/fn/floatnote` and under `~/Repos/Archive`
- **WHEN** the sync runs
- **THEN** the project `floatnote` is on `~/Repos/Apps/fn/floatnote`

#### Scenario: A GitHub key in mixed case

- **GIVEN** the Repository object with GitHub key `MultiChannable/multichannable`
- **AND** its checkout `~/Repos/Clients/mc/multichannable`, whose `origin` the lookup reads as `multichannable/multichannable`
- **WHEN** the sync runs
- **THEN** the project `multichannable` is on `~/Repos/Clients/mc/multichannable`

### Requirement: A repository that cannot be placed is reported and left alone

A repository with a Repository object that has no checkout, more than one checkout, a repository name another counting repository also has, or that GitHub does not know SHALL get no new project. The sync SHALL print one line naming it and the reason. An existing project of that repository SHALL stay as it is. This SHALL NOT count as a failure.

#### Scenario: No checkout

- **GIVEN** the Repository object `appboypov/work` and no checkout of it
- **WHEN** the sync runs
- **THEN** it prints that `appboypov/work` has no checkout
- **AND** no project `work` is made
- **AND** the run exits 0 when nothing else failed

#### Scenario: Two checkouts

- **GIVEN** a marked project for a repository that now has two checkouts outside `~/Repos/Archive`
- **WHEN** the sync runs
- **THEN** it prints both checkouts for that repository
- **AND** the project keeps its status and its resource

#### Scenario: Two repositories with one name

- **GIVEN** the Repository objects `appboypov/site` and `Supermarty-B-V/site`, each with one checkout, and no project `site`
- **WHEN** the sync runs
- **THEN** it prints both GitHub keys as sharing the name `site`
- **AND** no project `site` is made

### Requirement: A changed checkout is updated in place

When a marked project's `local_directory` resource differs from what the sync wants in `local_path`, `daemon_id` or `execution_mode`, the sync SHALL update that resource in place and keep the project, its id and its issues.

#### Scenario: The checkout moved

- **GIVEN** the marked project `rebl` on `~/Repos/Clients/rebl`
- **WHEN** the checkout moves to `~/Repos/Clients/rebl/rebl` and the sync runs
- **THEN** the same project now has that path as `local_path`
- **AND** its issues and status are unchanged

#### Scenario: A project switched to in place

- **GIVEN** a marked project whose resource was set to `execution_mode` `in_place` in Multica
- **WHEN** the sync runs
- **THEN** the resource is back in `worktree` mode

### Requirement: A project follows its repository out and back

When a Repository object is deleted, or GitHub archives the repository, the repository leaves. The sync SHALL move the marked project of a repository that left from `in_progress` to `completed`. The sync SHALL move the marked project of a counting repository from `completed` to `in_progress`. The sync SHALL NOT set any other status, and SHALL never delete a project.

#### Scenario: The Repository object is deleted

- **GIVEN** the marked project `photoprank` in status `in_progress`
- **WHEN** its Repository object is deleted and the sync runs
- **THEN** the project `photoprank` has status `completed`
- **AND** it still exists with its issues

#### Scenario: GitHub archives the repository

- **GIVEN** the marked project `mia` in status `in_progress`
- **WHEN** GitHub archives `appboypov/mia` and the sync runs
- **THEN** the project `mia` has status `completed`

#### Scenario: The repository returns

- **GIVEN** the marked project `photoprank` in status `completed` after its Repository object was deleted
- **WHEN** a Repository object for `appboypov/photoprank` is made again and the sync runs
- **THEN** the same project has status `in_progress`

#### Scenario: A status Brian set stays

- **GIVEN** a marked project of a counting repository that Brian set to `paused`
- **WHEN** the sync runs
- **THEN** the project is still `paused`

### Requirement: The sync manages only its own projects

A project SHALL be the sync's own when it has a `local_directory` resource labelled `madspec:<owner/name>`. The sync SHALL take over an unmarked project whose title equals a counting repository's name and that has no `local_directory` resource on the local daemon, by adding the marked resource. Every other project without the marker SHALL stay as it is. The sync SHALL NOT change a project's title, description, icon, lead, priority or issues.

#### Scenario: A hand-made project is taken over

- **GIVEN** an unmarked project `baba` without resources
- **WHEN** the sync runs
- **THEN** the same project `baba` has the marked `local_directory` resource on `~/Repos/Clients/baba/baba`
- **AND** no second project `baba` is made

#### Scenario: An unmarked project is left alone

- **GIVEN** an unmarked project `Experiments` whose name matches no Repository object
- **WHEN** the sync runs
- **THEN** `Experiments` keeps its status and resources

### Requirement: The sync writes nothing on a failed read

The sync SHALL read the Repository objects, GitHub's archived state and Multica's projects before it writes. When the brain, GitHub or Multica cannot be read, or the brain answers with no Repository object at all, it SHALL make no write, print what failed and exit non-zero. A failed write SHALL be printed, SHALL NOT stop the other projects, and SHALL make the run exit non-zero.

#### Scenario: The brain is unreachable

- **GIVEN** synced projects
- **AND** the Anytype API does not answer
- **WHEN** the sync runs
- **THEN** it prints that the brain cannot be read and exits non-zero
- **AND** no project changed status

#### Scenario: The brain answers with no Repository objects

- **GIVEN** synced projects
- **AND** the Anytype search returns no Repository object
- **WHEN** the sync runs
- **THEN** it prints that the brain holds no Repository objects and exits non-zero
- **AND** no project changed status

#### Scenario: Multica is unreachable

- **GIVEN** the Multica server is down
- **WHEN** the sync runs
- **THEN** it prints that Multica is unreachable and exits non-zero

### Requirement: A sync writes only what differs

The sync SHALL print one line per write and per repository it reports, and SHALL send a write only when Multica differs from what the brain and the checkouts say.

#### Scenario: A second run changes nothing

- **GIVEN** a run that just finished without failures
- **WHEN** the sync runs again with no change in the brain, GitHub or the checkouts
- **THEN** it sends no write to Multica

### Requirement: The sync runs by hand

`multica_projects.py` SHALL run only when started by hand. It SHALL read the brain and the checkouts with MadSpec's `team_state.py` from the MadSpec checkout `~/Repos/Ventures/madspec`, and talk to Multica with `multica_sync.py` from its own folder. When either cannot be loaded, it SHALL print that and exit non-zero, writing nothing.

#### Scenario: A run by hand

- **GIVEN** a Repository object for a repository with one checkout and no project
- **WHEN** `uv run madspec-sync/multica_projects.py` runs from the Multica checkout
- **THEN** the workspace holds the repository's project

### Requirement: Runs at once make one project

Runs of one Multica profile SHALL hold one file lock, `projects.lock` next to the profile's `config.json`, across their reads, plan and writes, so a run that starts while another runs waits for it and plans from what that run wrote.

#### Scenario: Two runs at once

- **GIVEN** a Repository object for a repository with one checkout and no project
- **WHEN** two runs start at the same time
- **THEN** both exit 0
- **AND** the workspace holds one project for that repository, with one `local_directory` resource
