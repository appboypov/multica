import type { Issue, IssueType } from "./issue";

export type IssueTemplateFieldFormat = "text" | "bullets" | "checks" | "scenarios";

export interface IssueTemplateField {
  key: string;
  label: string;
  required: boolean;
  placeholder: string;
  rows: number;
  format: IssueTemplateFieldFormat;
}

export interface IssueTemplateSection {
  heading: string;
  question: string;
  field?: IssueTemplateField | null;
}

export interface IssueTemplate {
  type: IssueType;
  sections: IssueTemplateSection[];
}

export interface IssueTemplatesResponse {
  templates: IssueTemplate[];
}

export interface SetIssueTypeRequest {
  type: IssueType;
}

/** Both pointers are required: send null to clear one. */
export interface SetIssuePlacementRequest {
  worklane_id: string | null;
  release_id: string | null;
}

export interface AddIssuesToReleaseRequest {
  issue_ids: string[];
}

export interface WorklaneIssuesResponse {
  issues: Issue[];
}

export interface ReleaseProgressRow {
  worklane_id: string;
  release_id: string;
  done: number;
  total: number;
}

export type IssuePlacementRefusalCode =
  | "type_invalid"
  | "type_fixed"
  | "release_needs_worklane"
  | "release_not_of_repository"
  | "release_shipped"
  | "issue_not_in_repository"
  | "issue_in_release"
  | "project_follows_worklane"
  | "worklane_has_open_issues"
  | "unknown";

export interface IssuePlacementRefusal {
  code: IssuePlacementRefusalCode;
  error?: string;
  version?: string;
  title?: string;
  repository?: string;
  issues: { id: string; identifier: string; title?: string }[];
}
