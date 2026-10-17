# Tasks

## At a glance

The [backend pull request](https://github.com/appboypov/multica/pull/84) targets `todo-inbox`. The [shared inbox pull request](https://github.com/appboypov/multica/pull/85) targets `todo-inbox-frontend` with the backend as a dependency.

- [x] 1. Store checks, project records, deterministic sample settings and history.
- [x] 2. Add recipient-only mode changes. Answers and counts keep the mode unchanged.
- [x] 3. Add instruction notes, Keep, Undo and agent undo reports with durable opener delivery.
- [x] 4. Add API and CLI contracts for check, record, instruction note and undo result paths.
- [ ] 5. Show record/history controls in project and check views, plus instruction notes in the inbox.
- [x] 6. Add service, handler, CLI, API and component tests for these paths.
- [ ] 7. Verify the named tests on Crabbox and validate with OpenSpec 1.13.0 in strict change mode.
