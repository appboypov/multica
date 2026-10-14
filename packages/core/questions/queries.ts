import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { QuestionRowFilters, QuestionTab } from "../types/question";

/** A parent issue or project whose Questions property lists what is below it. */
export type QuestionParent = NonNullable<QuestionRowFilters["parent"]>;

export const questionKeys = {
  all: (wsId: string) => ["questions", wsId] as const,
  byIssue: (wsId: string, issueId: string) => [...questionKeys.all(wsId), "issue", issueId] as const,
  details: (wsId: string) => [...questionKeys.all(wsId), "detail"] as const,
  detail: (wsId: string, id: string) => [...questionKeys.details(wsId), id] as const,
  /** Every row list of the Questions page, for invalidation. */
  rowsAll: (wsId: string) => [...questionKeys.all(wsId), "rows"] as const,
  rows: (wsId: string, tab: QuestionTab, filters: QuestionRowFilters) =>
    [...questionKeys.rowsAll(wsId), tab, filters] as const,
  openCount: (wsId: string) => [...questionKeys.all(wsId), "open-count"] as const,
  /** The open rows below a parent issue or project, under `rowsAll` so row invalidation reaches them. */
  parent: (wsId: string, parent: QuestionParent) =>
    [...questionKeys.rowsAll(wsId), "parent", parent.type, parent.id] as const,
  /** The open count below a parent, under `openCount` so count invalidation reaches it. */
  parentCount: (wsId: string, parent: QuestionParent) =>
    [...questionKeys.openCount(wsId), "parent", parent.type, parent.id] as const,
};

export function issueQuestionsOptions(wsId: string, issueId: string) {
  return queryOptions({
    queryKey: questionKeys.byIssue(wsId, issueId),
    queryFn: () => api.listIssueQuestions(issueId),
    enabled: !!wsId && !!issueId,
  });
}

export function questionDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: questionKeys.detail(wsId, id),
    queryFn: () => api.getQuestion(id),
    enabled: !!wsId && !!id,
  });
}

export function useIssueQuestions(wsId: string, issueId: string) {
  return useQuery(issueQuestionsOptions(wsId, issueId));
}

export function useQuestion(wsId: string, id: string) {
  return useQuery(questionDetailOptions(wsId, id));
}
