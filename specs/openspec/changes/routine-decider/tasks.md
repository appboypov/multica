# Tasks

## At a glance

Build the server decision path, committed event wiring and focused tests. Run checks on Crabbox with TypeSafe faked.

## Implementation

- [x] Add workspace rule parsing, typed TypeSafe choices and confidence gating.
- [x] Add persisted receipts and one transactional apply path for Jev and Decider answers.
- [x] Add Sonnet 5.5 task dispatch and completion handling.
- [x] Publish release-join events from every membership entry point and register decision listeners.
- [x] Add focused tests for confident apply, uncertain dispatch, completion and release-join wiring.
- [x] Add Brian's start-period question to-dos and atomic confirm/correct handling.
- [x] Keep Jev confidence and each agreed/corrected answer in the receipt and issue comments.
- [x] Guard workspace policy edits so only Brian can change the threshold or end the start period.
- [x] Select every fifth automatic choice in workspace order and reconcile corrections.
- [x] Add tests for confirmation, fallback confidence, policy access, samples, corrections and answer history.
- [ ] Validate the OpenSpec change and run scoped Go tests and build on Crabbox.
