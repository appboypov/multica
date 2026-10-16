# ADR-0021: A member asks a question on an issue, outside a run, and wakes no agent

- Status: accepted
- Date: 2026-10-16
- Supersedes: none

## Context

ADR-0002 made the question an aggregate with an asker agent and the run it was asked in, and the questions spec allowed only agents to ask. Brian's inbox moves from Anytype to Multica (WORK-435): everything he must pass, answer or do becomes a Multica question on the issue of the work. Sessions outside a Multica run, such as a Herdr omp session, hold a member login (Skuddy's) and no task token, so they cannot ask. Their answers have to come back to that session, which listens on the workspace's live connection, not to an agent run.

## Decision

- A question's asker is an agent in a run or a member outside one. `question` keeps `asker_agent_id` and `asked_in_task_id` for agent questions and gains `asker_user_id` for member questions; a CHECK constraint holds exactly one asker, and `asked_in_task_id` is set exactly when `asker_agent_id` is. Agent-only logic (run link, unseen answers, Wake, run input) keeps reading the agent columns and finds nothing on a member's question.
- A member asks through `POST /api/questions` and `multica question ask` with a member login; an agent asks only with its task token, and a machine credential (`cloud_pat`) never asks. The app still offers no way to ask.
- A member's question is asked on an issue: its first link is an issue of the workspace, and it has no run link. The issue's Agent questions setting does not apply, since ADR-0005 decides it for runs at claim.
- A member's question queues no run and reaches no running run as input. Its answers, ask backs and declines reach the asker the way every question event reaches a member: `question:updated` on the workspace's `/ws` broadcast.
- Only the asking agent's run closes, withdraws or links a question; a member's question stays open until its answer policy settles it.

## Consequences

- The answer, ask-back and Wake paths branch on `asker_agent_id` being set; ADR-0019 and ADR-0020 apply only to agent questions.
- Every view that names the asker reads one asker, agent or member, from the question; a member's question shows no run number.
- A listener that needs answers to a member's questions filters the workspace broadcast by question or ask id; no per-user channel is added.
- Letting a member close, withdraw or link its question later is an added permission on the same columns, not a new model.
