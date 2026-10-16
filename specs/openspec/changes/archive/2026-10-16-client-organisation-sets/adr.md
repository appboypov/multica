## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| Decision | Row-owned organisation sets | new | adr/0021-client-memberships-hold-organisation-sets.md |

## ADR review

- Status: completed
- Review date: 2026-10-16
- Relevant in-force constraints reviewed: `adr/0004-organisations-hold-projects.md` and `adr/0017-a-client-account-is-refused-outside-the-session-list.md`.
- The default-deny and one-membership account decisions remain in force. The organisation cardinality is specified by the new ADR without changing the client gate.
- Other ADRs concern issue planning, question delivery, releases or runtime worktrees and impose no additional storage or API contract here.

## Durable decision

`adr/0021-client-memberships-hold-organisation-sets.md` records the storage and cross-story API contract used by WORK-1071 and WORK-1072.
