import type { Attachment } from "./attachment";
import type { IssueWakeup, IssueWakeupInput, WakeupCondition } from "./issue-wakeup";
import type { WorklaneActivityEntry } from "./worklane";

export type ReleaseBump = "patch" | "minor" | "major";

export interface ReleaseMilestone {
  id: string;
  release_id: string;
  name: string;
  /** A calendar day, not a timestamp. */
  date: string;
  created_at: string;
  updated_at: string;
}

export interface ReleaseMilestoneRequest {
  name: string;
  date: string;
}

export type ReleaseOwnerType = "agent" | "squad";

/** Version has no leading `v`. Null bump means the version was set by hand. */
export interface Release {
  id: string;
  repository_id: string;
  position: number;
  version: string;
  bump: ReleaseBump | null;
  title: string;
  description: string;
  shipped_at: string | null;
  created_at: string;
  updated_at: string;
  milestones: ReleaseMilestone[];
  /** The optional agent or squad owner; every owner field is null without one. */
  owner_type: ReleaseOwnerType | null;
  owner_id: string | null;
  owner_name: string | null;
  owner_avatar_url: string | null;
}

/** Both null removes the owner. */
export interface ReleaseOwnerRequest {
  owner_type: ReleaseOwnerType | null;
  owner_id: string | null;
}

/** Plan and Edit send the owner pair or leave both out: Plan then has no owner, Edit keeps it. */
type ReleaseOwnerFields = { owner_type?: never; owner_id?: never } | ReleaseOwnerRequest;

export interface ReleaseDetailResponse {
  release: Release;
}

/** A pull request linked to an issue of the release, from GitHub or another VCS. */
export interface ReleaseIssuePullRequest {
  provider: string;
  number: number;
  url: string;
  title: string;
  state: string;
}

/** One issue of a release, in the release's order. */
export interface ReleaseIssue {
  id: string;
  identifier: string;
  title: string;
  status: string;
  assignee_type: "member" | "agent" | "squad" | null;
  assignee_id: string | null;
  parent_issue_id: string | null;
  /** Null for an issue whose worklane was removed. */
  worklane_id: string | null;
  worklane_name: string | null;
  pull_requests: ReleaseIssuePullRequest[];
}

export interface ReleaseIssuesResponse {
  issues: ReleaseIssue[];
}

export type ReleaseCommentAuthorType = "member" | "agent" | "system";

/** A comment or reply on a release's page. */
export interface ReleaseComment {
  id: string;
  release_id: string;
  author_type: ReleaseCommentAuthorType;
  author_id: string;
  content: string;
  parent_id: string | null;
  attachments: Attachment[];
  created_at: string;
  updated_at: string;
  source_task_id: string | null;
}

export interface CreateReleaseCommentRequest {
  content: string;
  parent_id?: string;
  attachment_ids?: string[];
}

/** A wait of the release's owner; `agent_id` is the owner of this moment, "" without one. */
export type ReleaseWakeup = Omit<IssueWakeup, "issue_id"> & { release_id: string };

/** Body of POST /api/releases/:id/wakeups: the release's owner of the firing moment runs it. */
export type ReleaseWakeupInput = Omit<IssueWakeupInput, "agent_id" | "event_types" | "filter_agent_id" | "filter_actor_type" | "filter_actor_id" | "condition"> & {
  condition?: Extract<WakeupCondition, { type: "other_issue" }>;
};

export interface ListReleasesResponse {
  releases: Release[];
}

/** Every release write answers with the repository's whole line. */
export interface ReleaseLineResponse {
  releases: Release[];
}

/** Send either a bump or an explicit version, never both. */
export type EditReleaseRequest = {
  title: string;
  description?: string;
} & (
  | { bump: ReleaseBump; version?: never }
  | { version: string; bump?: never }
) & ReleaseOwnerFields;

/** Plan release also places this worklane's issues in no release in the new release. */
export type PlanReleaseRequest = EditReleaseRequest & { issue_ids?: string[] };

export interface MoveReleaseRequest {
  direction: "earlier" | "later";
}

export interface ReleaseVersionChange {
  release_id: string;
  title: string;
  from: string;
  to: string;
}

export type ReleaseRefusalCode =
  | "title_required"
  | "version_or_bump"
  | "version_invalid"
  | "does_not_release"
  | "last_tag_unknown"
  | "first_release"
  | "last_release"
  | "line_not_increasing"
  | "release_shipped"
  | "release_before_shipped"
  | "release_not_next"
  | "release_not_ready"
  | "unknown";

export interface ReleaseRefusal {
  code: ReleaseRefusalCode;
  error?: string;
  release_id?: string;
  version?: string;
  title?: string;
  before?: { version: string; title: string } | { last_tag: string };
}

export interface ReleaseActivityDetails {
  release_id: string;
  version: string;
  title: string;
}

export interface ReleaseEditedDetails extends ReleaseActivityDetails {
  changes: ReleaseVersionChange[];
}

export interface ReleaseMovedDetails extends ReleaseEditedDetails {
  direction: "earlier" | "later" | "unknown";
}

export type ReleaseRemovedDetails = ReleaseEditedDetails;

/** Who owned the release before and after one owner change; null names no owner. */
export interface ReleaseOwnerChangedDetails extends ReleaseActivityDetails {
  owner_type: ReleaseOwnerType | null;
  owner_id: string | null;
  owner_name: string | null;
  previous_owner_type: ReleaseOwnerType | null;
  previous_owner_id: string | null;
  previous_owner_name: string | null;
}

export type ReleaseOwnerActivityEntry = Extract<WorklaneActivityEntry, { action: "release_owner_changed" }>;

/** The release's owner changes, oldest first. */
export interface ReleaseActivityResponse {
  entries: ReleaseOwnerActivityEntry[];
}
