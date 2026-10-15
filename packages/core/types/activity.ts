import type { CommentAuthorType, CommentSupplementReceipt, Reaction } from "./comment";
import type { Attachment } from "./attachment";
import type { IssueType } from "./issue";
import type { RunInputMark } from "./run-input";

export interface AssigneeFrequencyEntry {
  assignee_type: string;
  assignee_id: string;
  frequency: number;
}

export interface TimelineEntry {
  type: "activity" | "comment";
  id: string;
  actor_type: string;
  actor_id: string;
  created_at: string;
  /** Display identity hydrated from the actor's global user row when available. */
  actor_name?: string;
  actor_avatar_url?: string;
  // Activity fields
  action?: string;
  details?: Record<string, unknown>;
  /** Marks on this issue-change item, one per receiving run. */
  run_marks?: RunInputMark[];
  // Comment fields
  content?: string;
  parent_id?: string | null;
  updated_at?: string;
  revision?: number;
  comment_type?: string;
  /** Set only on comments a quick action produced (MUL-5465). Unforgeable. */
  quick_action_id?: string | null;
  reactions?: Reaction[];
  attachments?: Attachment[];
  resolved_at?: string | null;
  resolved_by_type?: CommentAuthorType | null;
  resolved_by_id?: string | null;
  source_task_id?: string | null;
  /** One receipt per comment version and running turn that received it. */
  supplements?: CommentSupplementReceipt[];
  /** Mirrors the first receipt; servers that predate `supplements` send only these. */
  supplement_task_id?: string;
  supplement_status?: "pending" | "delivering" | "delivered" | "failed";
  supplement_failure_reason?: string;
  supplement_delivered_at?: string;
  /**
   * Set only on a comment deleted while it still had replies: the server keeps
   * it as an empty tombstone so the replies keep their parent. Read it through
   * `isDeletedComment`.
   */
  deleted_at?: string | null;
  /** Set by frontend coalescing when consecutive identical activities are merged. */
  coalesced_count?: number;
}

/** Placement activity keeps names and versions as they were at the write. */
export interface IssueReleaseActivityDetails {
  release_id: string;
  version: string;
  title: string;
}

export interface IssueActivityDetailsByAction {
  created: { type?: IssueType; template?: boolean };
  planned_in_release: IssueReleaseActivityDetails & { worklane_id: string; worklane_name: string };
  type_set: { type: IssueType };
  added_to_release: IssueReleaseActivityDetails;
  removed_from_release: IssueReleaseActivityDetails;
  worklane_changed: { worklane_id: string | null; worklane_name: string | null };
  release_gone: IssueReleaseActivityDetails;
  label_added: { label_id: string; label_name: string };
  label_removed: { label_id: string; label_name: string };
  attachment_added: { attachment_id: string; filename: string };
  attachment_removed: { attachment_id: string; filename: string };
  sub_issue_added: { sub_issue_id: string; sub_issue_identifier: string; sub_issue_title: string };
  sub_issue_removed: { sub_issue_id: string; sub_issue_identifier: string; sub_issue_title: string };
}

export type IssueActivityAction = keyof IssueActivityDetailsByAction;

export type IssueActivityEntry = {
  [Action in IssueActivityAction]: Omit<TimelineEntry, "type" | "action" | "details"> & {
    type: "activity";
    action: Action;
    details: IssueActivityDetailsByAction[Action];
  };
}[IssueActivityAction];

export interface WorklaneSwitchedDetails {
  /** null or "" means the worktree was detached. */
  from: string | null;
  to: string | null;
  issue_id: string | null;
}

export interface WorklaneKeptDetails {
  branch: string;
  reason: "uncommitted" | "unpushed" | "unknown";
  count: number;
}
