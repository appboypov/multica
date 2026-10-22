# tickets Specification

## Purpose

The planning scripts of the `multica-platform` skill: `tickets.py` lists the stories that may start, `labels.py` keeps the ticket labels, and `team_state.py` collects the open work of an endeavour, a repository's intake and the workspace's In Review issues.

## Requirements

### Requirement: A story starts on done or stacked prerequisites
`tickets.py next` SHALL list a story as ready to start once each prerequisite is done or at most one of them is in review. A story builds on the branch of that one prerequisite's open pull request. An archived change SHALL count as done only once its archive folder is on the default branch. A whole-epic prerequisite SHALL hold the story until that epic is done.

#### Scenario: One prerequisite in review
- **GIVEN** story 2 waits on story 1
- **AND** story 1's change is in review
- **WHEN** `tickets.py next` runs
- **THEN** story 2 is ready to start on the branch of story 1's pull request

#### Scenario: Two prerequisites in review
- **GIVEN** story 3 waits on story 1 and story 2
- **AND** both changes are in review
- **WHEN** `tickets.py next` runs
- **THEN** story 3 is blocked

#### Scenario: An archive on the pull request's branch only
- **GIVEN** story 1's change is archived on its pull request's branch and the default branch lacks the archive
- **WHEN** `tickets.py next` runs
- **THEN** story 1 is in review

### Requirement: The ticket labels
`labels.py` SHALL report every ticket label the workspace lacks: `green light: Brian`, `risk:low`, `risk:medium`, `risk:high` and `hitl`. `--create` SHALL create exactly those, and leave existing labels as they are.

#### Scenario: Some labels exist
- **GIVEN** the workspace has `hitl` and `green light: Brian`
- **WHEN** `labels.py --create` runs
- **THEN** it creates every other ticket label
- **AND** it leaves the existing ones as they are
- **AND** a second run creates nothing

#### Scenario: Report only
- **GIVEN** the workspace has none of the labels
- **WHEN** `labels.py` runs
- **THEN** it lists every label as missing and creates nothing

#### Scenario: The planning tool cannot be reached
- **GIVEN** Multica fails
- **WHEN** `labels.py` runs
- **THEN** it exits with code 1 and an error on stderr

### Requirement: A story without Brian's green light is held
`tickets.py next --lights` SHALL move each story ready to start without Brian's green light to `held`.

#### Scenario: A story without the light is held
- **GIVEN** a ready story whose `green_lights` is empty
- **WHEN** `tickets.py next <folder> --lights` runs
- **THEN** the story is in `held`
- **AND** it is not in `ready_to_start`

### Requirement: The collector gathers all open work and checks nothing
`team_state.py` SHALL collect, for an endeavour folder, the repositories of its checkouts that the planning tool registers plus each repository the invocation names, and per repository its lanes in their order with each lane's state, branch, worktree path, issues in lane order, `active` and `next` issue with their `waits_on`, and its open issues the verifier holds for their release pull request; its release line with each release's milestones, open issues and done count; the projects those issues are in with their organisation and their open issues in no lane; the GitHub repositories the planning tool does not register; the OpenSpec changes and open pull requests of every checkout and worktree; and the latest client messages. It MUST NOT leave work out because of a field it does not expect. A source that cannot be read SHALL carry its error while everything else is still collected; only a failed read of the planning tool SHALL stop the run. The client messages SHALL be the latest WhatsApp messages, through Beeper Desktop, of every chat in the agent file of each client the endeavour's `AGENTS.md` names, and a chat it cannot read SHALL carry its error.

#### Scenario: Lanes in their order
- **GIVEN** repository `mc-app` with the lanes `main`, `Listing pipeline` and `Orders`, each with open issues
- **WHEN** the collector runs for MC's endeavour folder
- **THEN** it holds the three lanes in their order, each with its issues in lane order and its `active` and `next` issue

#### Scenario: Client feedback outside a lane
- **GIVEN** an open issue in MC's project that is in no lane and on no release
- **WHEN** the collector runs
- **THEN** it holds that issue under `unlaned`

#### Scenario: A repository the planning tool does not register
- **GIVEN** a GitHub checkout in the endeavour's folder that no project holds
- **WHEN** the collector runs
- **THEN** it names that repository under `unregistered` and collects everything else

#### Scenario: Beeper cannot be reached
- **GIVEN** Beeper Desktop is not running
- **WHEN** the collector runs for REBL's endeavour folder
- **THEN** each client chat carries its error, and the lanes, releases, changes and pull requests are still collected

#### Scenario: The planning tool cannot be read
- **GIVEN** Multica fails
- **WHEN** the collector runs
- **THEN** it exits with code 1 with the error

### Requirement: Repository intake ranks every open pull request
`team_state.py --intake <repository> --verifier <agent name>` SHALL list every open pull request of one repository with its linked issues, over all pages and every assignee, the pull requests with no issue included. It SHALL rank each pull request by its distinct open issues that wait on it transitively, then by board order, then by pull request number, into `ready` and `blocked`. Missing evidence SHALL stay visible as a blocker. A check rollup SHALL inform and SHALL NOT decide readiness.

#### Scenario: More than one page of work
- **GIVEN** open pull requests span three GitHub pages and one has no linked issue
- **WHEN** intake runs
- **THEN** every open pull request appears including the issue-less one

#### Scenario: One pull request blocks more work
- **GIVEN** two ready pull requests and two distinct open issues wait on the first
- **WHEN** intake ranks the repository
- **THEN** the first pull request comes before the second

#### Scenario: A failing check rollup
- **GIVEN** a ready pull request with a failed GitHub check rollup
- **WHEN** intake ranks the repository
- **THEN** the pull request stays in `ready`

### Requirement: In Review lists the verifier's work in merge order
`team_state.py --in-review` SHALL list every In Review issue of the workspace, in every project, with its project, release, lane, pull requests and place in its lane's stack. The rows SHALL come by project, then stack by stack with the bottom layer first, then the issues in no lane. `releases` SHALL hold each unshipped release whose issues are all closed and whose release pull request is open. The verifier SHALL start every session from this list, as `our-quality-conventions` `references/pull-request-verification.md` "Intake" says.

#### Scenario: A lane's stack
- **GIVEN** lane `Listing pipeline` with `WORK-56` and `WORK-57` In Review in that lane order
- **WHEN** `team_state.py --in-review` runs
- **THEN** `WORK-56` is `stack.position` 1 and `WORK-57` is `stack.position` 2
- **AND** `WORK-56` comes first

#### Scenario: A release with an open issue
- **GIVEN** release `0.1.1` with an open release pull request and one issue still In Review
- **WHEN** `team_state.py --in-review` runs
- **THEN** `releases` does not hold `0.1.1`
