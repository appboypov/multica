# ADR-0005: Project status and dates stay stored, and the web and desktop apps no longer use them

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

Brian does not use a project's status, start date or due date, and the PRD removes them from every page, dialog and filter (UJ-1). Several readers still depend on these fields: the `project` columns, the REST fields, the `multica project` CLI, the mobile app, the issue table's `project_statuses` filter and the `multica-platform` skill reference. The CLI and the mobile app are outside this work. This fork tracks upstream releases, and upstream keeps all of them.

## Decision

The database columns `project.status`, `start_date` and `due_date` stay. So do the API fields that carry them, the server-side filters and validation, and the wire types in `packages/core/types/project.ts` and `ProjectSchema`, which the mobile app imports.

The web and desktop apps stop using them, through the shared `packages/core` and `packages/views`:

- They have no store, column, sort, filter, badge, picker, icon or copy for them.
- Persisted view state that holds them is dropped when it loads.

Project search no longer hides or demotes a project by its status: not on the server, not in the client index, and not in the shared aggregation of search results. A status nobody can see then changes nothing anyone can see.

## Consequences

- Upstream syncs keep merging cleanly into the server and the CLI. The fork's own changes stay in the web and desktop layers and in project search.
- A project created in the web or desktop app gets the server's default status `planned`, which nothing shows.
- The mobile app and the CLI still show and set status and dates. Removing them there is its own change.
- A later change that drops the columns must first remove every reader listed in the Context.
