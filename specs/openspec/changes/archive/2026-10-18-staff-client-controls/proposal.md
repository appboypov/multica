## At a glance

```mermaid
sequenceDiagram
  actor Brian
  participant Members as Settings, Members
  participant Picker as Assignee picker
  participant API as Multica API
  actor Nasir as Nasir (client)
  Brian->>Members: Invite nasir@ivga.nl, role Client, IVGA Vastgoed and Whitebird
  Members->>API: POST members with role client and organisation_ids
  Nasir->>API: Accept the invitation
  Brian->>Members: Organisations, untick Whitebird
  Members->>API: PATCH member with the new organisation set
  Brian->>Picker: Assign "Send the 2025 rent roll" in an IVGA Vastgoed project
  Picker-->>Brian: Clients group lists Nasir
  Picker->>API: PATCH issue, assignee Nasir
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Capability | Staff client controls | new | `specs/openspec/specs/staff-client-controls/spec.md` |
| Capability | Client reads | modified | `specs/openspec/specs/client-reads/spec.md` |

## Why

Brian can only make a client contact through the CLI, and the staff screens read every client as a plain member. So nothing a client has to do can sit on that client's name from the app. Everything a client has to do, such as reviewing a prototype or delivering a DNS record or a login, is an ordinary issue assigned to that client's own member. Success is a client invited for A and B from Members, one organisation removed again, and a wait in A's project assigned to that client from the ordinary assignee picker.

## What Changes

- Invite member gets the role Client, with an Organisations field that takes one or more organisations of the workspace.
- Members and Pending invitations show the role Client and a badge per organisation of a client.
- A client's row menu gets Organisations, which adds or removes one organisation at once and never removes the last one.
- Change role keeps the staff roles; a client can be changed to a staff role.
- The assignee picker lists clients in their own Clients group, and only the clients of the issue's organisation: the organisation of the issue's project.
- The member profile shows the role Client.
- A client's waits are every open issue assigned to them in a project of their organisations, of any type and with any labels.
- The `multica-platform` skill finds a client wait by its client assignee, and its MadSpec planning reference says how a wait goes on the client's member.

## Capabilities

### New Capabilities

- `staff-client-controls`: staff invite and manage client contacts and assign issues to them in the existing member and assignee controls.

### Modified Capabilities

- `client-reads`: a client's own waits are the open issues assigned to them, whatever their type or labels.

## Impact

Shared web/desktop code: member and invitation types and zod schemas in `packages/core`, the members tab, the assignee picker and its call sites, the member profile in `packages/views` and their translations. Server: the client wait queries in `server/pkg/db/queries/client_read.sql` and their tests. The built-in `multica-platform` skill: `scripts/team_state.py`, its tests and the MadSpec planning reference. The membership API from WORK-1139 is used as published; no migration, mobile or client-screen change.

## Constraints

- The server stays the only authority: it refuses an empty organisation set, foreign organisations and every write from a client session.
- Staff roles, share links and assignment to staff, agents and squads behave as before.
- No client-contact directory and no client role on share links.

## Non-goals

Client screens (WORK-1073), the staff overview of waits (WORK-1074), mobile.

## Sources

- WORK-1072 description and Brian's comments of 18 October 2026.
- WORK-1059 "Plan", R1 and R5.
- Approved mock: `designs/screens/client-access.md`, screens 1, 2, 2b, 2c and 3.
- Membership API: `specs/openspec/changes/archive/2026-10-16-client-organisation-sets/design.md` "Published API contract".
