## At a glance

`OpenCheck` validates the requested member before updating the existing project and kind's recipient in its transaction.

## Context

`server/internal/service/check.go` inserts the first recipient with `ON CONFLICT DO NOTHING`. It then rejects every different recipient with 422. The unique key is `(workspace_id, project_id, kind)` in migration 790.

`lockCheckTrust` serializes opening and answering checks. `saveCheckTrust` updates counts without changing the recipient. `SetCheckSampling` authorizes the current record recipient. Individual check answers authorize the to-do's own recipient.

## Goals / Non-goals

Transfer the existing record through `CheckInput.MemberID`. Keep its counts and sampling schedule. Existing check recipients stay unchanged.

No API shape change or database migration. No new screen or endpoint.

## Decisions

Use an upsert that updates only `member_id` on the existing composite key. Remove the different-recipient refusal. Keep the row lock and existing sampling logic.

An explicit recipient in the existing open request is the handover action. The upsert rolls back with any failed check creation. Project and kind scope remain unchanged. Pending checks keep their own answer permissions and update the shared record.

## Verification

Use database-backed service regressions for sampling progress and immutable answer history. Verify transferred sampling authority and pending-check ownership. Verify nonmember rejection leaves the record unchanged. Exercise the existing HTTP handlers with both recipients.
