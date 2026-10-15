## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0017: A squad holds squads, and nesting is checked when it is written | new | `adr/0017-a-squad-holds-squads-and-nesting-is-checked-when-written.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-15

## Review Summary

No ADR in force governs squads. The member type, the write-time nesting checks and the direct-members-only default bind every later reader of `squad_member`, so they are one repository-level record.

## In-Force ADRs Reviewed

- ADR-0001 to ADR-0016: none touches squads, squad membership or comment routing; kept unchanged.

## New Durable ADRs Created

- `adr/0017-a-squad-holds-squads-and-nesting-is-checked-when-written.md`
