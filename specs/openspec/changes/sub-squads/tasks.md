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

- [x] 1.1 `server/migrations/745_squad_member_squad.up.sql`, `.down.sql` -- widen `squad_member_member_type_check` to `('agent', 'member', 'squad')` `NOT VALID` then `VALIDATE`; down deletes `squad` rows and restores the two-type CHECK -- design.md "Migration Plan".

## 2. Queries

- [x] 2.1 `server/pkg/db/queries/squad.sql` -- `LockSquadNesting` (workspace advisory lock), `ListSquadNestingMemberships` (every squad-in-squad membership in the workspace, in the order added), `ListSquadsNestedWithLeader` (direct non-archived parents and children of a squad led by an agent), `ListDelegatingParentSquads` (parents of a sub-squad whose current leader delegated to it on an issue); regenerate with `make sqlc` -- design.md decisions 1 to 4.

## 3. Services

- [x] 3.1 `server/internal/handler/squad_nesting.go` -- `squadNestingRefusal(ctx, qtx, parent, child)` returning a refusal message for a loop or the same leader, with `squadNestingLoop` walking the memberships breadth-first once per squad for the shortest loop path, and `squadLeaderNestingRefusal(ctx, qtx, squad, leaderID)` for a leader change -- design.md decisions 2 and 3.

## 4. Endpoints and routing

- [x] 4.1 `server/internal/handler/squad.go` -- `AddSquadMember` accepts `squad`: loads it in the workspace and not archived, applies `memberCanWireAgent` to its leader, takes the nesting lock in a transaction, runs `squadNestingRefusal`, inserts -- spec "A squad holds squads as members", "Nesting never makes a loop".
- [x] 4.2 `server/internal/handler/squad.go` -- `UpdateSquad` takes the nesting lock and runs `squadLeaderNestingRefusal` when `leader_id` changes -- spec "A sub-squad never shares its leader with its main squad".
- [x] 4.3 `server/internal/handler/squad_briefing.go` -- `renderMemberRow` renders `squad` rows and skips archived ones -- spec "The leader's roster lists sub-squads".
- [x] 4.4 `server/internal/handler/comment.go` -- `routeDelegatingParentSquadLeaders` in the agent branch of `computeCommentAgentTriggers`, between the guest and the assigned fallback -- design.md decision 4, spec "A sub-squad's report wakes the leader that delegated to it".

## 5. CLI and docs

- [x] 5.1 `server/cmd/multica/cmd_squad.go` -- `add`, `remove` and `set-role` accept `squad`; flag help names it -- proposal "What Changes".
- [x] 5.2 `server/internal/service/builtin_skills/multica-platform/references/squads.md` -- `member_type` lists `squad` and what a sub-squad row does -- agents read it.
- [x] 5.3 `apps/docs/content/docs/squads.mdx`, `squads.zh.mdx`, `squads.ja.mdx`, `squads.ko.mdx`, `squads.fr.mdx` -- members can be squads; loop and same-leader refusals; report-back -- AGENTS.md "UI Copy" keeps translations aligned.

## 6. Tests

- [x] 6.1 `server/internal/handler/squad_nesting_test.go` -- add a squad; direct, deep and self loop with the named path; a diamond nesting adds and refuses its loop with the shortest path; same leader; unknown and archived squad; leader change refused both ways; roster row and archived skip -- spec scenarios of the first four requirements.
- [x] 6.2 `server/internal/handler/squad_sub_squad_report_test.go` -- report wakes the delegating guest main leader; assigned main squad wakes once; no delegation wakes nobody; a mention in the report wakes only the mentioned -- spec "A sub-squad's report wakes the leader that delegated to it".

## Implementation Notes

- A sub-squad leader's report is a reply: a comment-triggered run cannot post top-level, so the report sits under its trigger and the guest worker fallback refuses it as a leader task before `routeDelegatingParentSquadLeaders` runs.
- "Two adds at the same time" rests on `LockSquadNesting`, one advisory lock per workspace taken before any squad row lock; no test races two transactions.

## Plan Change Log

- 2026-10-15: planned from Pieter's "Settled design: sub-squads" on WORK-1042; the report-back route reads the delegation instead of the reply parent (design.md decision 4), and the same-leader rule also guards leader changes (decision 3).
