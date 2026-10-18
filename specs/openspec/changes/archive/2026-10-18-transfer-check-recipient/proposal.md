## At a glance

Opening a check for another workspace member transfers that project and kind's recipient without replacing its record or history.

## Why

The first recipient is locked in permanently. Success means a later check reaches another workspace member while the existing project record and answers remain intact.

## What Changes

- Use the recipient of the latest opened check for the project and kind.
- Preserve counts and the sampling schedule during transfer.
- Preserve existing check ownership and answer history.

## Capabilities

### Modified Capabilities

- `check-todos`: allow recipient handover through the existing check-opening flow.

## Impact

The backend check service and its business-logic tests change. Existing API and CLI request shapes stay unchanged.

## Constraints

- Recipients remain workspace members.
- Each project and kind retains its existing record.

## Non-goals

No recipient settings screen or new endpoint. No reassignment of existing check to-dos. No sampling policy change.

## Sources

- WORK-1333 and its approved backend handoff.
- `server/internal/service/check.go`
- `server/internal/service/check_record.go`
