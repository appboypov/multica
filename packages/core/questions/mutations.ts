import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import type { AnswerQuestionRequest, AskBackQuestionRequest, ReassignQuestionRequest } from "../types/question";
import { questionKeys } from "./queries";

function useQuestionInvalidation(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return (id: string) => Promise.all([
    qc.invalidateQueries({ queryKey: questionKeys.byIssue(wsId, issueId) }),
    qc.invalidateQueries({ queryKey: questionKeys.detail(wsId, id) }),
  ]);
}

export function useAnswerQuestion(wsId: string, issueId: string) {
  const invalidate = useQuestionInvalidation(wsId, issueId);
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & AnswerQuestionRequest) => api.answerQuestion(id, data),
    onSettled: (_data, _error, { id }) => invalidate(id),
  });
}

export function useAskBackQuestion(wsId: string, issueId: string) {
  const invalidate = useQuestionInvalidation(wsId, issueId);
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & AskBackQuestionRequest) => api.askBackQuestion(id, data),
    onSettled: (_data, _error, { id }) => invalidate(id),
  });
}

export function useReassignQuestion(wsId: string, issueId: string) {
  const invalidate = useQuestionInvalidation(wsId, issueId);
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & ReassignQuestionRequest) => api.reassignQuestion(id, data),
    onSettled: (_data, _error, { id }) => invalidate(id),
  });
}

export function useWakeQuestionAgent(wsId: string, issueId: string) {
  const invalidate = useQuestionInvalidation(wsId, issueId);
  return useMutation({
    mutationFn: ({ id }: { id: string }) => api.wakeQuestionAgent(id),
    onSettled: (_data, _error, { id }) => invalidate(id),
  });
}

export function useUploadQuestionFile(wsId: string, issueId: string) {
  const invalidate = useQuestionInvalidation(wsId, issueId);
  return useMutation({
    mutationFn: ({ id, file, signal }: { id: string; file: File; signal?: AbortSignal }) =>
      api.uploadFile(file, { questionId: id }, signal),
    onSettled: (_data, _error, { id }) => invalidate(id),
  });
}
