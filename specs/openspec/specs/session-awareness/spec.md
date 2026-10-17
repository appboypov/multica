# session-awareness Specification

## Purpose
Agent sessions register with Multica, receive changed session lists and coordinate before changing the same repository and branch.

## Requirements

### Requirement: A started session registers with Multica

A session that an agent's command starts SHALL register with Multica as an active session of that agent, with its Herdr pane when it runs in Herdr, its folder, its repository and its branch. A session of an office with no Multica agent MUST NOT register: the receptionist's, since she is a member, and the AI engineer's until his agent exists.

#### Scenario: The product manager starts in Herdr

- **GIVEN** the tab `#mitchel` `#root`, in a worktree of `appboypov/madspec` on branch `work-1057`
- **WHEN** Brian runs `mitchel` there
- **THEN** `multica agent sessions mitchel` lists that session with its pane, its folder, `appboypov/madspec` and `work-1057`

#### Scenario: The AI engineer before his agent exists

- **GIVEN** Multica has no agent the AI engineer
- **WHEN** Brian runs `paul`
- **THEN** the session starts and registers nothing with Multica

### Requirement: A running session learns of a new one at its next step

A running session SHALL get the list of its agent's active sessions in its conversation at its next step whenever that list changed since it last got it, and MUST NOT get it when nothing changed.

#### Scenario: A second the product manager session starts

- **GIVEN** a product manager session runs in `#mitchel` `#root`
- **WHEN** a product manager run starts in the planning tool and the first session takes its next step
- **THEN** the first session's conversation holds the list with the new run in it

#### Scenario: Nothing changed

- **GIVEN** a product manager session got the list at its last step
- **WHEN** it takes its next step while no session started or ended
- **THEN** no list is added to its conversation

### Requirement: Coordinate before touching the same repository or branch

Before an agent's session changes a repository or branch, it SHALL check its agent's active sessions, and when another of them works on that repository and branch, it SHALL coordinate with that session before its first change there. The `multica-platform` skill's `references/madspec-planning.md` "Sessions" SHALL hold this rule, so it holds on a runtime that cannot add context at the next step too.

#### Scenario: Two sessions on one branch

- **GIVEN** a architect session works on branch `work-1057` of `appboypov/madspec`
- **WHEN** a second the architect session is about to commit on that branch
- **THEN** it contacts the first session before it commits

#### Scenario: Another branch

- **GIVEN** a architect session works on branch `work-1057` of `appboypov/madspec`
- **WHEN** a second the architect session is about to commit on `work-1060` of `appboypov/madspec`
- **THEN** it commits without contacting the first session
