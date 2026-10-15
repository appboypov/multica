## At a glance

```mermaid
sequenceDiagram
  actor B as Brian
  participant M as Multica server
  participant ML as Mitchel (leader of Tech)
  participant JL as Joeri (leader of Backend, a sub-squad of Tech)
  participant W as Backend worker
  B->>M: add squad Backend as a member of squad Tech
  M-->>B: refused when it makes a loop or Backend's leader is Tech's leader
  B->>M: assign or @mention Tech on an issue
  M->>ML: leader run; Squad Roster lists [@Backend](mention://squad/<id>)
  ML->>M: delegation comment @Backend
  M->>JL: leader run of Backend, as any @squad mention today
  JL->>M: delegation comment @worker
  M->>W: run
  W->>M: result comment
  M->>JL: leader run (today's worker-to-leader route)
  JL->>M: report comment, no @mention
  M->>ML: leader run of Tech, also when Tech is only a guest on the issue
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Model | `squad_member.member_type` gains `squad` | changed | `server/migrations/` |
| Endpoint | `POST /api/squads/{id}/members` takes `member_type: squad` | changed | `server/internal/handler/squad.go` |
| Endpoint | `PUT /api/squads/{id}` refuses a leader its parent or sub-squad has | changed | `server/internal/handler/squad.go` |
| Briefing | Squad Roster lists sub-squads | changed | `server/internal/handler/squad_briefing.go` |
| Routing | A sub-squad leader's report wakes the leader that delegated to it | changed | `server/internal/handler/comment.go` |
| CLI | `multica squad member add/remove/set-role --type squad` | changed | `server/cmd/multica/cmd_squad.go` |
| Docs | Squads page and the multica-platform squads reference | changed | `apps/docs/content/docs/squads*.mdx`, `server/internal/service/builtin_skills/multica-platform/references/squads.md` |

## Why

A squad can only hold agents and people today. A team of teams, such as a Tech squad over a Backend and a Frontend squad, has no way to hand work to a whole squad and let that squad's leader split it. Adding the sub-leader agent as a plain member runs it as a worker, without its squad briefing, so it never splits the work over its own team.

Brian approved Pieter's "Settled design: sub-squads" on WORK-1042 with one rule: a sub-squad behaves the same as a normal squad does today; only what nesting needs is added.

Success signal: Brian adds squad Backend to squad Tech; Tech's leader hands an issue to `@Backend`; Backend's leader splits it over its members; when Backend's leader reports back, Tech's leader wakes up, also when Tech was only @mentioned on the issue.

## What Changes

- A squad member can be a squad. The main squad's leader sees the sub-squad in its Squad Roster as an @squad mention and hands it work with that mention, which wakes the sub-squad's leader exactly as an @squad mention does today.
- Adding a squad as a member is refused when it would create a loop at any depth, with a message that names the loop, and when the sub-squad has the same leader as the main squad. One squad may sit in several main squads; depth is not limited.
- Changing a squad's leader is refused when the new leader leads a squad it sits in or a squad that sits in it, so the same-leader rule cannot be bypassed afterwards.
- When the sub-squad's leader posts a report without an @mention, the leader of the main squad that delegated to it on that issue wakes up, also when the main squad is only a guest on the issue.
- An archived sub-squad stays a member and is left out of the roster, as an archived agent is.
- The CLI's squad member commands take `--type squad`.

## Capabilities

### New Capabilities

- `squads`: lets a squad hold other squads as members and hand them work through their leaders.

## Impact

- Server: one migration widening a CHECK, squad member add, squad leader change, roster, comment routing.
- CLI: `multica squad member` commands.
- Docs: squads page in five languages, the built-in multica-platform squads reference.
- Web, desktop and mobile: no change in this change; the UI change follows as its own change (picker, member list, `SquadMemberType`, zod fallback).

## Constraints

- @squad inbox notices, squad questions and "involves me" reach direct members of the named squad only, as today; they never look into sub-squads.
- No foreign key or cascade for the new member type (AGENTS.md): the handler validates the sub-squad in the workspace.
- Older desktop clients read a `squad` member type they do not know; the UI change adds the zod fallback.

## Non-goals

- A depth cap, a hop limit or loops allowed at runtime.
- The main leader reaching sub-squad members directly.
- Status, presence or skills of a sub-squad in the members-status view or roster beyond its name, role and mention.

## Sources

- WORK-1042 comment "Settled design: sub-squads" (Pieter), approved by Brian in a Plannotator Review on 15 Oct 2026.
- WORK-1042 comment "Recommendation: a squad as a third member type" (Pieter): the options compared and the gaps.
