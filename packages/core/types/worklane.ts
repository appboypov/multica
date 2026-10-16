import type {
  ReleaseActivityDetails,
  ReleaseEditedDetails,
  ReleaseMovedDetails,
  ReleaseRemovedDetails,
  ReleaseOwnerChangedDetails,
} from "./release";
import type { WorklaneSwitchedDetails, WorklaneKeptDetails } from "./activity";

/**
 * Worklanes — the lanes of work of a repository (ADR-0006).
 *
 * Every repository has its `main` worklane, made with it: `is_main` true, no
 * `folder`, `daemon_id` or `path`, because it works in the repository's
 * checkout. Every other worklane has its own worktree on one machine. Names are
 * unique per repository, ignoring case.
 */

/** Whether an issue of the worklane is being worked on. */
export type WorklaneState = "free" | "busy";

export interface WorklaneIssueRef {
  id: string;
  identifier: string;
  title: string;
  status: string;
}

export interface Worklane {
  id: string;
  workspace_id: string;
  repository_id: string;
  name: string;
  /** The worktree folder under the repository's Worktrees folder; null for `main`. */
  folder: string | null;
  /** The machine of the worktree; null for `main`. */
  daemon_id: string | null;
  /** The absolute worktree path the daemon resolved; null for `main`. */
  path: string | null;
  is_main: boolean;
  position: number;
  state: WorklaneState;
  branch: string | null;
  active_issue: WorklaneIssueRef | null;
  next_issue: WorklaneIssueRef | null;
  created_at: string;
  updated_at: string;
}

export interface ListWorklanesResponse {
  worklanes: Worklane[];
  total: number;
}

export interface CreateWorklaneRequest {
  repository_id: string;
  name: string;
  folder: string;
  daemon_id: string;
}

export interface RenameWorklaneRequest {
  name: string;
}

/**
 * Making, removing or switching a worklane runs as a request on its machine,
 * polled until it ends. Every enum below has `unknown` for a value from a newer
 * server, shown as a failed step with the server's detail.
 */
export type WorklaneRequestKind = "create" | "remove" | "switch" | "unknown";

export type WorklaneRequestStatus = "running" | "failed" | "completed" | "refused" | "unknown";

export type WorklaneRequestStepKey = "fetch" | "add_worktree" | "install" | "open" | "check" | "switch" | "unknown";

export type WorklaneRequestStepState = "pending" | "running" | "done" | "skipped" | "failed" | "unknown";

export interface WorklaneRequestStep {
  key: WorklaneRequestStepKey;
  state: WorklaneRequestStepState;
  detail: string;
  /** The last 8 KB of the step's output. */
  output: string;
  duration_ms: number;
}

/** Why a worklane request was refused or failed. */
export type WorklaneRequestReasonCode =
  | "name_required"
  | "name_taken"
  | "folder_invalid"
  | "folder_taken"
  | "runtime_no_checkout"
  | "runtime_offline"
  | "runtime_cannot_make_worktrees"
  | "install_command_unknown"
  | "request_running"
  | "main_fixed"
  | "path_issue_worktree"
  | "path_not_checkout"
  | "lost"
  | "busy"
  | "uncommitted"
  | "unpushed"
  | "stashed"
  | "worklane_has_open_issues"
  | "unknown";

export interface WorklaneRequestReason {
  code: WorklaneRequestReasonCode;
  detail: string;
  count?: number;
}

export interface WorklaneRequest {
  id: string;
  kind: WorklaneRequestKind;
  repository_id: string;
  /** Set once `open` saved the worklane, and on a remove request. */
  worklane_id: string | null;
  name: string;
  folder: string;
  path: string;
  /** `origin/{default_branch}`. */
  base: string;
  install_command: string;
  daemon_id: string;
  /** The machine's name when the request was made. */
  runtime_name: string;
  status: WorklaneRequestStatus;
  steps: WorklaneRequestStep[];
  reason: WorklaneRequestReason | null;
  created_by: string | null;
}

/**
 * Worklane activity is `activity_log` rows of the worklane, and the
 * repository-wide rows that show on every worklane of the repository
 * (ADR-0008). An action from a newer server is `unknown` and not rendered.
 */
export type WorklaneActivityAction =
  | "worklane_created"
  | "worklane_renamed"
  | "release_planned"
  | "release_edited"
  | "release_moved"
  | "release_removed"
  | "release_shipped"
  | "release_owner_changed"
  | "worklane_switched"
  | "worklane_kept"
  | "unknown";

export interface WorklaneCreatedDetails {
  name: string;
  runtime_name: string;
}

export interface WorklaneRenamedDetails {
  from: string;
  to: string;
}

export interface WorklaneActivityEntryBase {
  id: string;
  actor_type: string;
  actor_id: string | null;
  created_at: string;
}

export type WorklaneActivityEntry = WorklaneActivityEntryBase &
  (
    | { action: "worklane_created"; details: WorklaneCreatedDetails }
    | { action: "worklane_renamed"; details: WorklaneRenamedDetails }
    | { action: "release_planned" | "release_shipped"; details: ReleaseActivityDetails }
    | { action: "release_edited"; details: ReleaseEditedDetails }
    | { action: "release_moved"; details: ReleaseMovedDetails }
    | { action: "release_removed"; details: ReleaseRemovedDetails }
    | { action: "release_owner_changed"; details: ReleaseOwnerChangedDetails }
    | { action: "worklane_switched"; details: WorklaneSwitchedDetails }
    | { action: "worklane_kept"; details: WorklaneKeptDetails }
    | { action: "unknown"; details: Record<string, unknown> }
  );

/** The newest 200 entries, oldest first. */
export interface WorklaneActivityResponse {
  entries: WorklaneActivityEntry[];
}
