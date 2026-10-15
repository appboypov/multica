import type { IssuePriority, IssueStatus } from "./issue";

export type InboxSeverity = "action_required" | "attention" | "info";

export type InboxItemType =
  | "issue_assigned"
  | "issue_subscribed"
  | "unassigned"
  | "assignee_changed"
  | "status_changed"
  | "priority_changed"
  | "start_date_changed"
  | "due_date_changed"
  | "new_comment"
  | "mentioned"
  | "review_requested"
  | "task_completed"
  | "task_failed"
  | "agent_blocked"
  | "agent_completed"
  | "reaction_added"
  | "quick_create_done"
  | "quick_create_failed"
  // Quick create whose outcome could not be verified. Distinct from
  // quick_create_failed because it must NOT be rendered with failure framing:
  // the issue may actually have been created.
  | "quick_create_unconfirmed"
  // System notifications are intentionally issue-less. Keep them in the
  // same Inbox model so read/archive/realtime behavior remains consistent.
  | "autopilot_paused"
  | "autopilot_quota_exceeded"
  // Sub-issues of an issue assigned to the recipient closed (the child_done
  // system rule notifies a member assignee instead of waking an agent).
  | "children_done"
  // A question an agent asked the recipient; `details.question_id` names it.
  // Never muted: none is in a notification group.
  | "question_asked"
  | "question_reassigned"
  | "question_overdue";

/**
 * One workspace's unread notice count in the cross-workspace summary
 * (`GET /api/inbox/unread-summary`): the Updates meaning. Web and desktop
 * badges count to-dos instead (`TodoWorkspaceCount`).
 */
export interface InboxWorkspaceUnread {
  workspace_id: string;
  count: number;
}

export interface InboxItem {
  id: string;
  workspace_id: string;
  recipient_type: "member" | "agent";
  recipient_id: string;
  actor_type: "member" | "agent" | "system" | null;
  actor_id: string | null;
  type: InboxItemType;
  severity: InboxSeverity;
  issue_id: string | null;
  title: string;
  body: string | null;
  issue_status: IssueStatus | null;
  /**
   * Current priority of the linked issue. Optional so an installed Desktop
   * client remains compatible with an older backend that predates this Inbox
   * projection; null also covers notifications without a linked issue.
   */
  issue_priority?: IssuePriority | null;
  read: boolean;
  archived: boolean;
  created_at: string;
  details: Record<string, string> | null;
}


export interface ArchivedInboxPage {
  items: InboxItem[];
  nextCursor: string | null;
  hasMore: boolean;
}

export interface ArchivedInboxFacets {
  statuses: Record<string, number>;
  priorities: Record<string, number>;
  actors: Record<string, number>;
  unreadCount: number;
}
