import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "../api";

export const questionKeys = {
  all: (wsId: string) => ["questions", wsId] as const,
  byIssue: (wsId: string, issueId: string) => [...questionKeys.all(wsId), "issue", issueId] as const,
  details: (wsId: string) => [...questionKeys.all(wsId), "detail"] as const,
  detail: (wsId: string, id: string) => [...questionKeys.details(wsId), id] as const,
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
