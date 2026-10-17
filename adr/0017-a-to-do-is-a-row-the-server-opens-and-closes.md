# ADR-0017: A to-do is a row the server opens and closes

- Status: accepted
- Date: 2026-10-15
- Supersedes: none

## Context

WORK-449 turns the inbox into the to-dos of a member: things that wait for the member to act, each leaving by itself once handled, with Mark done by hand. Their sources are many tables: questions and their answerers, comments, issues with their assignee, creator and status, agent runs, autopilots and quota periods. Mark done must hold until a later event asks again. The Inbox number shows on every page, in the switcher and on the desktop badge.

## Decision

- `todo` keeps one row per to-do. `internal/todo` opens a row when the event that makes it happens and closes it, with `done_by = handled`, when the event that handles it happens. Mark done closes it with `done_by = member`.
- A to-do is open while `done_at` is NULL. One open row per `(workspace_id, member_id, kind, source_key)`; a repeated open or close changes nothing.
- Fields that change on their source, such as the issue's title, status and due date and the question's type, are read from the source when listing.
- Every change to the member's visible list publishes `todo:opened` or `todo:closed` to that member. An open question hides the member's review on its issue without completing it, and closing the last question reveals it. Hiding publishes `todo:closed` with a null `done_by`, and a review that opens or closes while hidden publishes nothing.

## Consequences

- Lists and counts are plain reads of open rows, cheap enough for every page.
- Each new to-do source needs its open and close wired to an event. A missed close leaves a row that Mark done clears.
- Today's open to-dos are backfilled once from the sources when the table arrives.
