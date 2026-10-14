import type { QueryClient, QueryFilters } from "@tanstack/react-query";
import type { Issue } from "../types/issue";
import type { Question } from "../types/question";
import { issueKeys } from "../issues/queries";
import { questionKeys } from "./queries";

export async function invalidateIssueQuestions(qc: QueryClient, wsId: string, issueId: string, questionId?: string) {
  const filters: QueryFilters[] = [
    { queryKey: questionKeys.byIssue(wsId, issueId) },
    {
      queryKey: questionKeys.details(wsId),
      predicate: (query) => {
        if (questionId && query.queryKey[3] === questionId) return true;
        const question = query.state.data as Question | undefined;
        // An unresolved detail has no link information yet; refetching avoids
        // keeping an in-flight snapshot from before this event indefinitely.
        return !question || question.links.some((link) => link.link_type === "issue" && link.link_id === issueId);
      },
    },
  ];
  // Initial loads have no cached data, so invalidation alone reuses the
  // pre-event request. Cancel first, as inbox refreshes do, then fetch anew.
  await Promise.all(filters.map((filter) => qc.cancelQueries(filter)));
  await Promise.all(filters.map((filter) => qc.invalidateQueries(filter)));
}

/** The Questions page's rows and the sidebar count: refetched, never patched. */
export function invalidateQuestionRows(qc: QueryClient, wsId: string) {
  return Promise.all([
    qc.invalidateQueries({ queryKey: questionKeys.rowsAll(wsId) }),
    qc.invalidateQueries({ queryKey: questionKeys.openCount(wsId) }),
  ]);
}

/** Question and task events carry identities, not authoritative question data. */
export function handleQuestionEvent(qc: QueryClient, wsId: string, type: string, payload: unknown) {
  if (type !== "question:created" && type !== "question:updated" && !type.startsWith("task:")) return;
  if (!wsId || !payload || typeof payload !== "object") return;
  // Events are workspace-wide; every question event may move the viewer's rows.
  const rows = type.startsWith("question:") ? invalidateQuestionRows(qc, wsId) : undefined;
  const fields = payload as Record<string, unknown>;
  if (typeof fields.issue_id !== "string" || !fields.issue_id) return rows;
  // `issue_ids` names every issue whose page shows the card; `issue_id` is its first.
  const issueIds = new Set([fields.issue_id]);
  if (Array.isArray(fields.issue_ids)) {
    for (const id of fields.issue_ids) if (typeof id === "string" && id) issueIds.add(id);
  }
  const questionId = typeof fields.question_id === "string" ? fields.question_id : undefined;
  return Promise.all([rows, ...[...issueIds].flatMap((issueId) => [
    invalidateIssueQuestions(qc, wsId, issueId, questionId),
    // Wake writes activity without publishing activity:created. Question
    // updates and issue task events are the live signal for other viewers.
    type !== "question:created"
      ? qc.invalidateQueries({ queryKey: issueKeys.timeline(issueId) })
      : undefined,
  ])]);
}

/**
 * What reaches what, and who is offered Wake, follows the live issue tree
 * (ADR-0007). Call before the issue caches are patched: the change is read
 * against the cached issue, and an issue that is not cached counts as changed.
 */
export function onIssueReachMaybeChanged(
  qc: QueryClient,
  wsId: string,
  issue: Partial<Issue> & Pick<Issue, "id">,
  flags: { assigneeChanged?: boolean; projectChanged?: boolean },
) {
  const cached = qc.getQueryData<Issue>(issueKeys.detail(wsId, issue.id));
  const changed = !cached
    || flags.assigneeChanged === true
    || flags.projectChanged === true
    || (issue.parent_issue_id !== undefined && issue.parent_issue_id !== cached.parent_issue_id)
    || (issue.project_id !== undefined && issue.project_id !== cached.project_id)
    || (issue.assignee_type !== undefined && issue.assignee_type !== cached.assignee_type)
    || (issue.assignee_id !== undefined && issue.assignee_id !== cached.assignee_id);
  if (changed) return invalidateQuestionReach(qc, wsId);
}

/** Deleted issues and any project change move what reaches what. */
export function invalidateQuestionReach(qc: QueryClient, wsId: string) {
  return qc.invalidateQueries({ queryKey: questionKeys.all(wsId) });
}
