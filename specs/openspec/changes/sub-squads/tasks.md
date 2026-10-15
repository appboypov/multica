## At a glance

One team, Backend, builds every group in one pull request. The UI is the Frontend team's change after this one.

| Group | Kind | Builder |
|---|---|---|
| 1 | model change | `madspec-model-builder` |
| 2 | queries | `madspec-model-builder` |
| 3 | services | `madspec-service-builder` |
| 4 | endpoints and routing | `madspec-api-builder` |
| 5 | CLI and docs | `madspec-command-builder` |
| 6 | tests | `madspec-service-builder` |

## 1. Model change

- [ ] 1.1 `server/migrations/732_squad_member_squad.up.sql`, `.down.sql` -- widen `squad_member_member_type_check` to `('agent', 'member', 'squad')` `NOT VALID` then `VALIDATE`; down deletes `squad` rows and restores the two-type CHECK -- design.md "Migration Plan".

## 2. Queries

- [ ] 2.1 `server/pkg/db/queries/squad.sql` -- `LockSquadNesting` (workspace advisory lock), `FindSquadNestingLoop` (recursive path from the added squad to the main squad), `ListSquadsNestedWithLeader` (direct non-archived parents and children of a squad led by an agent), `ListDelegatingParentSquads` (parents of a sub-squad whose current leader delegated to it on an issue); regenerate with `make sqlc` -- design.md decisions 1 to 4.

## 3. Services

- [ ] 3.1 `server/internal/handler/squad_nesting.go` -- `checkSquadNesting(ctx, qtx, parent, child)` returning a refusal message for a loop or the same leader, and `checkLeaderAgainstNesting(ctx, qtx, squad, leaderID)` for a leader change -- design.md decisions 2 and 3.

## 4. Endpoints and routing

- [ ] 4.1 `server/internal/handler/squad.go` -- `AddSquadMember` accepts `squad`: loads it in the workspace and not archived, applies `memberCanWireAgent` to its leader, takes the nesting lock in a transaction, runs `checkSquadNesting`, inserts -- spec "A squad holds squads as members", "Nesting never makes a loop".
- [ ] 4.2 `server/internal/handler/squad.go` -- `UpdateSquad` takes the nesting lock and runs `checkLeaderAgainstNesting` when `leader_id` changes -- spec "A sub-squad never shares its leader with its main squad".
- [ ] 4.3 `server/internal/handler/squad_briefing.go` -- `renderMemberRow` renders `squad` rows and skips archived ones -- spec "The leader's roster lists sub-squads".
- [ ] 4.4 `server/internal/handler/comment.go` -- `routeDelegatingParentSquadLeaders` in the agent branch of `computeCommentAgentTriggers`, between the guest and the assigned fallback -- design.md decision 4, spec "A sub-squad's report wakes the leader that delegated to it".

## 5. CLI and docs

- [ ] 5.1 `server/cmd/multica/cmd_squad.go` -- `add`, `remove` and `set-role` accept `squad`; flag help names it -- proposal "What Changes".
- [ ] 5.2 `server/internal/service/builtin_skills/multica-platform/references/squads.md` -- `member_type` lists `squad` and what a sub-squad row does -- agents read it.
- [ ] 5.3 `apps/docs/content/docs/squads.mdx`, `squads.zh.mdx`, `squads.ja.mdx`, `squads.ko.mdx`, `squads.fr.mdx` -- members can be squads; loop and same-leader refusals; report-back -- AGENTS.md "UI Copy" keeps translations aligned.

## 6. Tests

- [ ] 6.1 `server/internal/handler/squad_nesting_test.go` -- add a squad; direct, deep and self loop with the named path; same leader; other workspace and archived squad; leader change refused both ways; roster row and archived skip -- spec scenarios of the first four requirements.
- [ ] 6.2 `server/internal/handler/squad_sub_squad_report_test.go` -- report wakes the delegating guest main leader; assigned main squad wakes once; no delegation wakes nobody; a mention in the report wakes only the mentioned -- spec "A sub-squad's report wakes the leader that delegated to it".

## Implementation Notes

## Plan Change Log

- 2026-10-15: planned from Pieter's "Settled design: sub-squads" on WORK-1042; the report-back route reads the delegation instead of the reply parent (design.md decision 4), and the same-leader rule also guards leader changes (decision 3).
