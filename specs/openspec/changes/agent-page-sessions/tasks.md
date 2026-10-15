## At a glance

| Group | Kind | Team | Builder |
|---|---|---|---|
| 1. Sessions data | service | Frontend | `madspec-service-builder` |
| 2. Sessions section | screen | Frontend | `madspec-view-builder` |

The branch `work-1089` stacks on `work-1087` (`agent-active-sessions`), which stacks on `work-1057` (`agent-session-model`). Group 2 reads group 1's query.

## 1. Sessions data

- [ ] 1.1 `packages/core/api/client.ts` -- `listAgentSessions(agentId, {signal})` as design.md decision 1, beside `listAgentTasksPage`.
- [ ] 1.2 `packages/core/agents/queries.ts` -- `agentSessionsKeys` and `agentSessionsOptions(wsId, agentId)`; `packages/core/agents/index.ts` already exports the file.
- [ ] 1.3 `packages/core/realtime/use-realtime-sync.ts` -- invalidate `agentSessionsKeys.all(wsId)` in the `agent` and `task` refreshes, as design.md decision 2.
- [ ] 1.4 `packages/core/agents/queries.test.ts` -- cached sessions are kept apart by workspace and agent; a malformed response (no `sessions`, an unknown `status`) gives a usable list -- the page relies on both.

## 2. Sessions section

- [ ] 2.1 `packages/views/agents/components/tabs/agent-sessions-list.tsx` -- `AgentSessionsList({agent})` as design.md decision 3: one row per session with where, issue link, folder, repository, branch, status and start time; `EmptyText`-style "No active sessions".
- [ ] 2.2 `packages/views/agents/components/tabs/activity-tab.tsx` -- a first `Section` "Sessions" with its subtitle, holding `AgentSessionsList`.
- [ ] 2.3 `packages/views/locales/{en,fr,ja,ko,zh-Hans}/agents.json` -- `activity.section_sessions`, `activity.subtitle_sessions_one`, `_other`, `activity.subtitle_no_sessions`, `activity.empty_sessions`, `activity.session_run_on`, `activity.session_unknown_where` and `activity.session_status.{idle,working,blocked,done,starting,unknown}`, translated in each locale.
- [ ] 2.4 `packages/views/agents/components/tabs/activity-tab.render.test.tsx` -- a Herdr session and a run render with their pane, "run on" runtime, linked issue, folder, repository, branch and status; no sessions shows "No active sessions"; after an `agent:sessions` invalidation without the session, its row is gone -- the three scenarios of `agent-page-sessions`.
