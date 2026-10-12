# ADR-0004: A question set groups questions; open is per question and per answerer

- Status: accepted
- Date: 2026-10-12
- Supersedes: none

## Context

Brian's PRD lets an agent ask several questions in one ask, of several people, under the answer policy Any one or Each, and counts open questions per question, not per ask: a set of three open questions is "3 open". Four journeys read these facts: UJ-2 draws the set card and the Questions row, UJ-3 lists and counts what is open for the viewer (For me, the sidebar count, One by one, One page), and UJ-4 counts questions on every parent. If each journey groups sets and decides "open" in its own way, the same issue shows different counts in different places. ADR-0002 makes the question its own aggregate of five tables; a set has to fit it without reshaping it.

## Decision

A question set is a group key on `question`: `ask_id` is the id of the ask's first question, `ask_position` its place, `ask_title` the set's title. A one-question ask has `ask_id = id`. There is no ask table. The answerers, answer policy, due date and Blocks work of an ask are copied to each of its questions, so every rule, count and list works on single questions.

A question's status follows from one rule over its policy, its current answerers and their answers: Any one is `answered` from the first answer; Each is `partly_answered` until every current answerer has answered, then `answered`. "Open" means status `open` or `partly_answered`. "Open for a member" means open, the member is a current answerer, and the member has no answer to it. The rule and both definitions live in `server/internal/question/policy.go`, mirrored in `packages/core/questions/cards.ts`; every count and list of open questions uses them, one per question.

## Consequences

- A set is shown as one card by grouping on `ask_id`; nothing else in the model knows about sets.
- UJ-3's For me lists and the sidebar count are "open for a member"; the Questions row, run badges and UJ-4's parent counts are "open". None of them counts asks.
- A new answer policy changes the rule in one place and adds a value to `answer_policy`, not a table.
- Copied fields cannot drift because a question is never edited; reassigning is per question.
