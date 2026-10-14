import { ISSUE_TYPES, type Issue, type IssueType } from "../types";
import { issueTypePicked } from "../issues/surface/membership";

/** A pick of the Type filter: one of the eight types, or `none` for No type. */
export type IssueTypePick = IssueType | "none";

/** The Type filter's options in menu order: the eight types, then No type. */
export const ISSUE_TYPE_PICKS: readonly IssueTypePick[] = [...ISSUE_TYPES, "none"];

/**
 * The Type filter as a predicate for the issues of Timeline and List, by the
 * rule the server applies to the Board's `filters.types`: no pick keeps every
 * issue, and `none` keeps the issues without a type.
 */
export function typeFilter(types: readonly IssueTypePick[]): (issue: Pick<Issue, "type">) => boolean {
  return (issue) => issueTypePicked(issue.type, types);
}

/** How many of the issues each pick of the Type filter keeps. */
export function typeCounts(issues: readonly Pick<Issue, "type">[]): Record<IssueTypePick, number> {
  const counts = Object.fromEntries(ISSUE_TYPE_PICKS.map((pick) => [pick, 0])) as Record<IssueTypePick, number>;
  for (const issue of issues) counts[issue.type ?? "none"] += 1;
  return counts;
}
