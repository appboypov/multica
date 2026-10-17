import type { IssueStatus } from "./issue";

export const TODO_KINDS = [
  "question", "mention", "assigned", "agent_blocked", "run_failed",
  "autopilot_paused", "quota_reached", "quick_create_failed", "review",
] as const;

export type TodoKind = (typeof TODO_KINDS)[number];

export interface Todo {
  id: string;
  workspace_id: string;
  kind: TodoKind;
  issue_id: string | null;
  issue_identifier: string | null;
  issue_title: string | null;
  issue_status: IssueStatus | null;
  question_id: string | null;
  question_answer_type: string | null;
  comment_id: string | null;
  task_id: string | null;
  autopilot_id: string | null;
  actor_type: "member" | "agent" | null;
  actor_id: string | null;
  title: string;
  body: string | null;
  details: Record<string, unknown>;
  due_date: string | null;
  overdue: boolean;
  created_at: string;
  done_at: string | null;
  done_by: "handled" | "member" | null;
}

/** Account-level open counts; workspaces with zero to-dos are omitted. */
export interface TodoWorkspaceCount {
  workspace_id: string;
  count: number;
}

export interface TodoOpenedPayload {
  member_id: string;
  todo: Todo;
}

export interface TodoClosedPayload {
  member_id: string;
  todo_id: string;
  kind: TodoKind;
  /** Null when the to-do left without being done, such as a review hidden by an open question. */
  done_by: "handled" | "member" | null;
}
