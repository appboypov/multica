# ADR-0017: A chat turn may ask by the workspace default

- Status: accepted, supersedes ADR-0005
- Date: 2026-10-15
- Supersedes: ADR-0005 (`0005-whether-a-run-may-ask-is-decided-at-claim.md`)

## Context

ADR-0005 decides once, at claim, whether a run may ask questions, and turns asking off for every task without an issue. Brian decided on WORK-447 (2026-10-15) that an agent in a Multica chat asks the same structured questions as on issues, that the workspace Questions default decides it, and that a chat has no Questions setting of its own. A chat turn has no issue, so ADR-0005's "a task without an issue is off" no longer holds for chat turns. Chats connected from Slack, Lark/Feishu or WeCom are out of scope: their member cannot see question cards.

## Decision

Whether a run may ask is decided once per run, when its task is claimed, and stored on `agent_task_queue.questions_enabled` under the claim CAS:

- An issue task: `service.ResolveAgentQuestions(issue.agent_questions, workspace.settings)`, the issue's value when set, else the workspace default, absent meaning On.
- An in-app chat task (`chat_session_id` set, no channel context): `service.ResolveAgentQuestions(nil, workspace.settings)`, the workspace default alone.
- Every other task, including channel chat turns, quick-create and autopilot runs without an issue: off.

The claim sends the value to the daemon as `questions_enabled`. The brief, the daemon's environment, the CLI and the skill variant follow the claimed value; `POST /api/questions` refuses a task whose stored value is false, and a task that is neither an issue task nor an in-app chat task. Product-owned text an off run must not see sits between `<!-- questions -->` markers in built-in skills, and an off run gets the skill variant without it, under its own skill id.

## Consequences

- A change of the workspace default reaches the next claimed chat turn, never a running one.
- A later way to ask, or new text about questions in the brief, the CLI or a built-in skill, follows the claimed value or the markers.
- Channel chat turns stay off until a change gives their member a way to answer.
- Tasks claimed before this decision keep their stored value; chat tasks claimed earlier have `questions_enabled` false and cannot ask, which fails closed.
