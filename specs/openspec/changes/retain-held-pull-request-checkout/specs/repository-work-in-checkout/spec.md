## ADDED Requirements

### Requirement: A held open pull request branch stays in the checkout

With Work in the project folder off, a task on the checkout's machine SHALL work in that checkout when its recorded branch matches the selected linked open pull request branch in the issue's workspace and repository. This SHALL apply to issues without a worklane and to issues in another worklane. The claim SHALL identify the main worklane with `InCheckout` and preserve the pull request branch.

#### Scenario: Held pull request branch with the setting off

- **GIVEN** the checkout holds `review/actual-head` for the issue's linked open pull request
- **AND** Work in the project folder is off
- **WHEN** the next task on that issue is claimed on the checkout's machine
- **THEN** its claim selects the checkout's main worklane with `InCheckout`
- **AND** the task works in the checkout on `review/actual-head`

#### Scenario: An unrelated pull request does not retain the checkout

- **GIVEN** Work in the project folder is off and the checkout holds `review/actual-head`
- **AND** no selected linked open pull request in the issue's workspace and repository matches that branch
- **WHEN** a task on the issue is claimed
- **THEN** the task keeps its existing worklane or issue-worktree route
