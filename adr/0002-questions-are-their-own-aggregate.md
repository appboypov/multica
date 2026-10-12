# ADR-0002: Questions are their own aggregate with typed answers and links

- Status: accepted
- Date: 2026-10-12
- Supersedes: none

## Context

Agents ask people questions during their runs (PRD: Questions in Multica). Five journeys build on the same question: UJ-1 asks one question on the issue and run it was asked in, UJ-2 adds question sets and answer policies, UJ-3 lists every open question on one page, UJ-4 links questions to any workspace object and shows them on every parent, UJ-5 turns asking off per issue. A question must not be a comment, it has twelve answer types with their own settings and values, and only people answer.

## Decision

A question is an aggregate of its own tables: `question` (asker agent, the run it was asked in, prompt, answer type, `settings` and `suggested_answer` as JSONB, status, due date, Blocks work), `question_link` (`link_type`, `link_id`, `position`; position 0 is where it was asked), `question_answerer` (the current member answerers), `question_answer` (one row per answerer, `value` as JSONB, edited in place) and `question_event` (its history: asked, answered, answer edited, ask back, reassigned, closed, withdrawn, woken). Answer types, their settings and their values are validated by one Go registry, `server/internal/question`; the client mirrors it only to gate sending. Status moves `open` → `answered` → `closed`, with `withdrawn` from `open` or `answered`; overdue is derived from the due date, never stored as a status.

## Consequences

- A new link kind (project, comment, sub-issue) is a new `link_type` value and its loaders, never a new table or column.
- Question sets and answer policies add columns or a set table next to `question`; answers, answerers and history keep their shape.
- A new answer type is a registry entry plus its client control; the server's validation is the one that counts.
- Lists of questions for a person or a link query `question_answerer` and `question_link`; their indexes come with the journey that adds the list.
- Comments, the timeline API and `TimelineEntry` stay untouched; clients merge questions into views themselves.
