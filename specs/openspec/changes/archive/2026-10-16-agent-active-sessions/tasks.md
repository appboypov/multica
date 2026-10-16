## At a glance

| Group | Kind | Team | Builder |
|---|---|---|---|
| 1. Endpoints | endpoint | Backend | `madspec-api-builder` |
| 2. Daemon session follower | integration | Backend | `madspec-service-builder` |

Group 2 calls group 1's routes. The branch `work-1087` stacks on `work-1057` (`agent-session-model`).

## 1. Endpoints

- [x] 1.1 `server/internal/handler/agent_session.go` -- `RegisterAgentSession`, `UpdateAgentSession`, `EndAgentSession`, `ListDaemonAgentSessions` and `ListAgentSessions` as design.md decision 4: workspace and `daemon_id` checks, `parseUUIDOrBadRequest` for ids, the agent checked against the workspace, `agent:sessions` published after each write.
- [x] 1.2 `server/cmd/server/router.go` -- the four routes under `/api/daemon/agent-sessions` in the `DaemonAuth` group (1557-1617), `GET /sessions` in the `/api/agents/{id}` route beside `GET /tasks` (2322).
- [x] 1.3 `server/internal/handler/agent_session_test.go` -- with `testPool`: register then list shows the session; a second register of the same PID replaces it; update and end by another daemon are refused and change nothing; end of an unknown id is `204`; a running task of the agent is listed as a run and leaves once completed; an agent of another workspace is `404`; each write publishes `agent:sessions` with the agent's id -- the routes are where workspace and daemon ownership are enforced.

## 2. Daemon session follower

- [x] 2.1 `server/internal/daemon/client.go` -- `RegisterAgentSession`, `UpdateAgentSession`, `EndAgentSession`, `ListAgentSessions` over the routes of 1.2.
- [x] 2.2 `server/internal/daemon/agent_sessions.go` -- the follower of design.md decisions 1 to 3: the followed set keyed by session id with pane, PID, start time, folder, repository, branch and status; `register` (process start time, `origin` as `owner/name`, branch); the Herdr subscriber on `HERDR_SOCKET_PATH` or `~/.config/herdr/herdr.sock` with reconnect every 30 s; the 30-second sweep; the start-up load per served workspace. Folder reads and process checks go through small interfaces so tests can fake them.
- [x] 2.3 `server/internal/daemon/health.go` -- `POST /sessions` on the local listener: decode, `400` for a workspace the daemon does not serve or a PID that does not run, else register and answer `201 {id}`. `server/internal/daemon/daemon.go` -- start the follower in `Run` and stop it on shutdown.
- [x] 2.4 `server/internal/daemon/agent_sessions_test.go` -- with a fake Herdr socket, fake process and git readers and a recording client: a status event of a followed pane updates once, an unchanged status sends nothing, a branch change in the sweep is reported; `pane.closed` and `pane.exited` end the session; an event of an unknown pane, the daemon's own included, changes nothing; a PID that is gone, or reused with another start time, ends the session; start-up ends rows whose process is gone; a lost socket reconnects and the sweep goes on -- these are the ways a session ends or goes stale.
