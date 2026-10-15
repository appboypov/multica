import type { Todo } from "@multica/core/types";

/** A Question to-do on WORK-412 for the inbox's component tests. */
export function todoFixture(overrides: Partial<Todo> = {}): Todo {
  return {
    id: "todo-1",
    workspace_id: "ws-1",
    kind: "question",
    issue_id: "issue-412",
    issue_identifier: "WORK-412",
    issue_title: "VAT rounding",
    issue_status: "in_progress",
    question_id: "q-1",
    question_answer_type: "single_choice",
    comment_id: null,
    task_id: null,
    autopilot_id: null,
    actor_type: "agent",
    actor_id: "agent-1",
    title: "VAT rounding",
    body: null,
    details: {},
    due_date: null,
    overdue: false,
    created_at: new Date().toISOString(),
    done_at: null,
    done_by: null,
    ...overrides,
  };
}
