## At a glance

| Group | Kind | Team | Builder |
|---|---|---|---|
| 1. Session commands | CLI command | Agent Tools | `madspec-command-builder` |
| 2. Persona runtime profiles | CLI command | Agent Tools | `madspec-command-builder` |

The branch `work-1088` stacks on `work-1087` (`agent-active-sessions`). Group 2 is independent of group 1.

## 1. Session commands

- [x] 1.1 `server/cmd/multica/cmd_agent_sessions.go` -- `agent sessions <agent>` as design.md decision 1: `resolveAgent`, `GET /api/agents/{id}/sessions`, the table WHERE, FOLDER, REPOSITORY, BRANCH, ISSUE, STATUS, STARTED, "No active sessions." when empty, `--output json` as received.
- [x] 1.2 `server/cmd/multica/cmd_agent_sessions.go` -- `agent register-session <agent> --pid --cwd [--pane]` as design.md decision 2: `requireHumanLocalCommand`, `resolveAgent`, `POST http://127.0.0.1:<healthPortForProfile(resolveProfile(cmd))>/sessions`, the session id printed, the "daemon is not running" message on a refused connection, the daemon's message on `400`.
- [x] 1.3 `server/cmd/multica/cmd_agent.go` -- add both commands to `agentCmd`.
- [x] 1.4 `server/cmd/multica/cmd_agent_sessions_test.go` -- with `httptest` servers as `cmd_agent_test.go` does: a Herdr session and a run render with their pane and `run on <runtime>`; an empty list prints "No active sessions." and JSON `"sessions": []`; `register-session` posts the agent's id, PID, folder and pane to the daemon port; a closed daemon port fails with the daemon message and posts nothing; a `400` returns the daemon's message -- these are what the persona command and office extension rely on.

## 2. Persona runtime profiles

- [x] 2.1 `madspec-sync/multica_sync.py` -- personas from `roster.py --personas`, members left out; per persona the profile of design.md decision 3: listed, created when no profile has the alias as `command_name`, its path pinned with `multica --profile <daemon profile> runtime profile set-path` when the daemon profile's CLI config lacks it; the daemon profile from `multica daemon status --output json`, the whole sync failing before any change when no daemon runs.
- [x] 2.2 `madspec-sync/multica_sync.py` -- `sync_agent` and `update_agent` move the agent to the local runtime whose `profile_id` is its profile, failing that persona with "start the daemon, then run the sync again" when none exists; `instruction_selection` `[]`, `instructions` empty, `custom_args` only the `--config=` overlay; `omp_runtime()` removed. Seed new agents' model and thinking level from their current persona configuration; omit both fields from existing-agent updates so Multica edits survive later syncs.
- [x] 2.3 `madspec-sync/specs/multica-agents.md` -- "An agent runs its persona as the persona command does" says the agent runs on its runtime profile through its alias; the selection requirements go.
- [x] 2.4 `madspec-sync/tests/test_multica_sync.py` -- with `FakeMultica` and a fake daemon status: Mitchel gets a profile `mitchel` of type omp and moves onto its runtime; an existing profile is reused, not duplicated; a deleted profile is created again; Paul's approved shared persona creates an active agent on its profile, preserves later model/thinking edits and leaves Roos archived; Tika, Herdr, Astra, Fable and Haiku get no profile and keep their runtime; a profile without a registered runtime fails only that persona; no running daemon fails before any request that writes; the agent's selection, instructions and `--skills=` are cleared and its `--config=` overlay kept. The tests of `omp_runtime()` and of the whole skill folder selection go -- these are the ways a persona can end on the wrong runtime.
  Cover a new agent's initial model and thinking level, then edits to both in Multica surviving a later sync even when the persona configuration differs.

## Implementation Notes

- Session registration resolves relative folders before sending them to the daemon. Required registration tests select an available profile port and never skip.
- The office roster reads current office configuration, matching the alias. The sync uses the checkout's shared skills directly; the office symlinks make a Git archive unsuitable for this source.
- Daemon discovery checks the default profile, then named profiles serving this server and workspace. Profile runtimes are matched by both profile and daemon.
- The Product sync owns its checkout-runtime lookup and selects the built-in omp runtime, excluding the persona profiles this change creates. Its transport import stays in the persona sync.
- Verification on the Mac: seven focused Go tests, `go vet ./cmd/multica`, CLI build, 30 persona-sync tests and the Product runtime-selection regression passed. Crabbox SSH bootstrap timed out after 180 seconds.
- Executable smoke against isolated API/daemon fixtures passed: mixed and empty session output, registration, daemon rejection and absence; actual sync entrypoint created the persona profile and seeded its model, then preserved Multica model/thinking edits on the next run. No live workspace sync ran.
- The broader Product sync test fixture references `multica_projects.subprocess`, which the existing module does not expose. That unrelated fixture failure remains outside this change.
- Office/persona integration: 48 MadSpec roster/alias tests (plus four subtests) and 30 persona-sync tests passed with the companion shared roster loaded. The Mac CLI fixture requires canonical `TMPDIR=/private/tmp`; Crabbox availability timed out after 30 seconds.
- Isolated full-sync entrypoint ran twice with the approved shared persona: Paul was created on his alias runtime, later Multica model/thinking edits survived, and Roos stayed archived. Alias entrypoint loaded the unchanged approved skill and report capability. Live-office discovery passed read-only; no live workspace sync or deployment ran.
