# ADR-0006: Questions are listed per viewer through one server-side row list

- Status: accepted
- Date: 2026-10-12
- Supersedes: none

## Context

Brian's PRD gives every member one place for the questions that wait on them: a sidebar item **Questions** with a count, a Questions page with the tabs For me, All open and Answered, and **Answer all**, which works through the viewer's open questions. UJ-4 adds the tab On my work and lists and counts questions on parents and for the indirect audience. ADR-0004 fixes what "open" means: per question and per answerer, `open` or `partly_answered`, the viewer a current answerer without an answer of their own.

Questions are reachable today only per issue (`GET /api/issues/{id}/questions`, UJ-1). Building the page from those reads would make the client fetch every issue a viewer might be asked on and decide which question belongs in which tab, so the sidebar count, the tabs and Answer all could each decide differently, and the count would need every issue loaded. Mobile, which also reads questions later, would repeat that rule a third time.

## Decision

Which questions a viewer sees on the Questions page, and how many wait on them, is decided by the server, in one list:

- `GET /api/questions?tab=…` returns question **rows** for the viewer, scoped by tab, with filters and a cursor. A row is a projection (the question, its issue link with identifier, title and status, and the viewer's own answer), not the full question card.
- `GET /api/questions/open-count` returns the For me count, from the same rule as the For me tab.
- The SQL of both repeats ADR-0004's rule; a parity test runs the same fixtures through `question.IsOpenFor` and the list, so the two cannot drift.
- The client only groups, orders and displays rows. It never filters rows into tabs or counts them itself. Answering and waking use UJ-1's per-question endpoints, and Answer all reads full questions through UJ-1's per-issue read.
- A new journey that lists questions per viewer adds its tab or link type to this endpoint (UJ-4: `on_my_work`, parents and the indirect audience), never a second list.

## Consequences

- The sidebar count, the For me tab and Answer all's session agree, because they come from the same rule in one place.
- Three concurrent indexes serve the lists: the viewer's answerer rows, open questions per workspace, and the viewer's answers by time. No table or column is added.
- A row is not a card: opening or answering a question beyond Yes / No goes to UJ-1's card data, so row and card shapes evolve apart.
- UJ-4 extends this endpoint instead of building its own list; its parent and audience rules live next to ADR-0004's rule in SQL.
- Mobile can use the same endpoints when it shows questions.
