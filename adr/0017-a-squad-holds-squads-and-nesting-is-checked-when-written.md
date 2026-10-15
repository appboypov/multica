# ADR-0017: A squad holds squads, and nesting is checked when it is written

- Status: accepted
- Date: 2026-10-15
- Supersedes: none

## Context

Squads need sub-squads: a team of teams hands work to a whole squad and lets its leader split it. Options were a `squad` member type, flattening sub-squad members into the main squad, a `parent_squad_id` column, and adding the sub-leader agent as a plain member. Brian approved the member type on WORK-1042 with the rule that a sub-squad behaves as a squad does today, plus only what nesting needs.

## Decision

- `squad_member.member_type` takes `squad`; `member_id` is then a squad of the same workspace. One squad may sit in several squads; depth is not limited.
- A main leader reaches a sub-squad only through its `@squad` mention, which wakes the sub-squad's leader as any `@squad` mention does.
- Loops and a sub-squad sharing its leader with its main squad are refused when a member is added or a leader changes, under one workspace advisory lock (`<workspace_id>:squad_nesting`). Stored data never holds them, so readers need no loop guard.
- Every reader of `squad_member` other than the leader roster and the sub-squad report-back route reads the direct `agent` and `member` rows only.

## Consequences

- A new reader of squad membership decides whether it reads direct members only; the default is direct only.
- Every write that adds a `squad` member or changes a leader takes the nesting lock first.
- Clients that know only `agent` and `member` meet a third type and need a fallback.
