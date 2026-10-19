## ADDED Requirements

### Requirement: Disabled rules stay in the policy
A decider rule with `disabled` set SHALL stay in the workspace policy and SHALL start no decision on any event.

#### Scenario: Disabled squad rules ask nothing
- **GIVEN** disabled squad rules on entering todo and on release_joined
- **WHEN** an unassigned top-level issue enters todo and joins a release
- **THEN** no routine decision or question is made
- **AND** the issue keeps no assignee

### Requirement: Workspace squad rules are disabled on deployment
Deployment SHALL retain and disable `todo-squad` and `release-squad` in the De App Specialist workspace. It SHALL preserve `post-merge-bug` and all other settings. Other workspaces SHALL remain unchanged.

#### Scenario: Deployment stops squad questions
- **GIVEN** the De App Specialist workspace has enabled `todo-squad` and `release-squad` rules
- **WHEN** the deployment applies its data migration
- **THEN** both rules remain stored with `disabled` set to true
- **AND** entering Todo or joining a release creates no squad decision or question
- **AND** the post-merge bug rule still works
