import type { Project, ProjectPriority } from "../types";
import type { ProjectSortDirection, ProjectSortField } from "./stores/view-store";

// Sort order map for the priority column (header sort needs a total order).
const PRIORITY_ORDER: Record<ProjectPriority, number> = {
  urgent: 4,
  high: 3,
  medium: 2,
  low: 1,
  none: 0,
};

/**
 * Orders projects by the project view store's sort: Projects, an
 * organisation's page, Worklanes and Releases all list projects in this order.
 * Callers sort before they group, because `groupProjectsByOrganisation` keeps
 * the input order in each group.
 */
export function sortProjects(
  projects: readonly Project[],
  field: ProjectSortField,
  direction: ProjectSortDirection,
): Project[] {
  const dir = direction === "asc" ? 1 : -1;
  return [...projects].sort((a, b) => {
    if (field === "name") return a.title.localeCompare(b.title) * dir;
    if (field === "priority") {
      return (
        (PRIORITY_ORDER[a.priority] - PRIORITY_ORDER[b.priority]) * dir ||
        a.title.localeCompare(b.title)
      );
    }
    if (field === "progress") {
      const progressA = a.issue_count > 0 ? a.done_count / a.issue_count : -1;
      const progressB = b.issue_count > 0 ? b.done_count / b.issue_count : -1;
      return (progressA - progressB) * dir || a.title.localeCompare(b.title);
    }
    return (Date.parse(a.created_at) - Date.parse(b.created_at)) * dir;
  });
}
