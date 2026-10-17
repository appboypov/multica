import type { QueryClient, QueryFilters, QueryKey } from "@tanstack/react-query";
import type { Issue } from "../types/issue";
import type { Question, QuestionLinkType } from "../types/question";
import { issueKeys } from "../issues/queries";
import { questionKeys } from "./queries";
import type { QuestionScope } from "./scope";

/**
 * Refetches a list of linked questions and the cached questions linked to the
 * same object; without a list only the unresolved details and `questionId`.
 */
async function refetchLinkedQuestions(
  qc: QueryClient,
  wsId: string,
  list: { key: QueryKey; link: { type: QuestionLinkType; id: string } } | null,
  questionId?: string,
) {
  const filters: QueryFilters[] = [
    ...(list ? [{ queryKey: list.key }] : []),
    {
      queryKey: questionKeys.details(wsId),
      predicate: (query) => {
        if (questionId && query.queryKey[3] === questionId) return true;
        const question = query.state.data as Question | undefined;
        // An unresolved detail has no link information yet; refetching avoids
        // keeping an in-flight snapshot from before this event indefinitely.
        return !question || (!!list && question.links.some((entry) => entry.link_type === list.link.type && entry.link_id === list.link.id));
      },
    },
  ];
  // Initial loads have no cached data, so invalidation alone reuses the
  // pre-event request. Cancel first, as inbox refreshes do, then fetch anew.
  await Promise.all(filters.map((filter) => qc.cancelQueries(filter)));
  await Promise.all(filters.map((filter) => qc.invalidateQueries(filter)));
}

export function invalidateIssueQuestions(qc: QueryClient, wsId: string, issueId: string, questionId?: string) {
  return refetchLinkedQuestions(qc, wsId, { key: questionKeys.byIssue(wsId, issueId), link: { type: "issue", id: issueId } }, questionId);
}

export function invalidateChatQuestions(qc: QueryClient, wsId: string, chatSessionId: string, questionId?: string) {
  return refetchLinkedQuestions(qc, wsId, { key: questionKeys.byChat(wsId, chatSessionId), link: { type: "chat", id: chatSessionId } }, questionId);
}

/** The questions of the issue or chat a card shows in; a question without either refreshes on its own. */
export function invalidateScopeQuestions(qc: QueryClient, wsId: string, scope: QuestionScope, questionId?: string) {
  if ("chatSessionId" in scope) return invalidateChatQuestions(qc, wsId, scope.chatSessionId, questionId);
  return scope.issueId
    ? invalidateIssueQuestions(qc, wsId, scope.issueId, questionId)
    : refetchLinkedQuestions(qc, wsId, null, questionId);
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
  const questionId = typeof fields.question_id === "string" ? fields.question_id : undefined;
  // A question asked in a chat shows in that chat, also when it is on an
  // issue; a chat turn's task events move whether its agent is offered Wake.
  const chat = typeof fields.chat_session_id === "string" && fields.chat_session_id
    ? invalidateChatQuestions(qc, wsId, fields.chat_session_id, questionId)
    : undefined;
  if (typeof fields.issue_id !== "string" || !fields.issue_id) return Promise.all([rows, chat]);
  // `issue_ids` names every issue whose page shows the card; `issue_id` is its first.
  const issueIds = new Set([fields.issue_id]);
  if (Array.isArray(fields.issue_ids)) {
    for (const id of fields.issue_ids) if (typeof id === "string" && id) issueIds.add(id);
  }
  return Promise.all([rows, chat, ...[...issueIds].flatMap((issueId) => [
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
  return changed ? invalidateQuestionReach(qc, wsId) : Promise.resolve();
}

/** Deleted issues and any project change move what reaches what. */
export function invalidateQuestionReach(qc: QueryClient, wsId: string) {
  return qc.invalidateQueries({ queryKey: questionKeys.all(wsId) });
}
