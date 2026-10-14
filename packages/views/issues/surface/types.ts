import type { IssueScope } from "@multica/core/issues/surface/scope";
import type { IssueOpenMode } from "@multica/core/issues/stores/issue-opening-store";
import type { CreateIssueRequest } from "@multica/core/types";
import type { ViewMode } from "@multica/core/issues/stores/view-store";

export type IssueCreateDefaults = Partial<
  Omit<
    CreateIssueRequest,
    "assignee_type" | "assignee_id" | "parent_issue_id" | "project_id"
  >
> & {
  assignee_type?: CreateIssueRequest["assignee_type"] | null;
  assignee_id?: string | null;
  parent_issue_id?: string | null;
  /** Display-only context for the create dialog while the parent query loads. */
  parent_issue_identifier?: string;
  project_id?: string | null;
};

export type IssueSurfaceMode = Extract<
  ViewMode,
  "board" | "list" | "table" | "swimlane" | "gantt"
>;

export interface IssueSurfaceProps {
  scope: IssueScope;
  modes: IssueSurfaceMode[];
  surfaceKey?: string;
  createDefaults?: IssueCreateDefaults;
  /** Server-owned membership search shared by non-Table issue surfaces. */
  search?: string;
  /** What a plain click on a card or row opens; the device's open mode when unset. */
  openMode?: IssueOpenMode;
  /** Toast "{identifier} moved to {status}" after the server accepts a moved card's status. */
  announceMoves?: boolean;
}
