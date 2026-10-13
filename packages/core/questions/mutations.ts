import { useMutation, useQueryClient } from "@tanstack/react-query";
import { EMPTY_QUESTION_DRAFT, questionDraftKey, useQuestionDraftStore, type QuestionDraft } from "./draft-store";
import { api } from "../api";
import type { AnswerQuestionRequest, AskBackQuestionRequest, ReassignQuestionRequest } from "../types/question";
import { invalidateIssueQuestions } from "./ws-updaters";
import { issueKeys } from "../issues/queries";

function useQuestionInvalidation(wsId: string, issueId: string) {
  const qc = useQueryClient();
  return (id: string) => invalidateIssueQuestions(qc, wsId, issueId, id);
}

/**
 * Mutation keys per question, so every card showing the question sees its
 * send as pending, also a card mounted again after the send started.
 */
export const questionMutationKeys = {
  answer: (wsId: string, questionId: string) => ["questions", wsId, "answer", questionId] as const,
  askBack: (wsId: string, questionId: string) => ["questions", wsId, "ask-back", questionId] as const,
};

export function useAnswerQuestion(wsId: string, issueId: string, questionId: string) {
  const invalidate = useQuestionInvalidation(wsId, issueId);
  const draftKey = questionDraftKey(wsId, questionId);
  return useMutation({
    mutationKey: questionMutationKeys.answer(wsId, questionId),
    mutationFn: (data: AnswerQuestionRequest) => api.answerQuestion(questionId, data),
    // The draft as it was sent, so the success clears only that.
    onMutate: (): QuestionDraft => useQuestionDraftStore.getState().drafts[draftKey] ?? EMPTY_QUESTION_DRAFT,
    // The sent answer or edit is done with even when its card scrolled away
    // before the server replied; a per-call callback would not run then.
    onSuccess: (_data, _variables, sent) => useQuestionDraftStore.getState().clearSentAnswer(draftKey, sent),
    onSettled: () => invalidate(questionId),
  });
}

export function useAskBackQuestion(wsId: string, issueId: string, questionId: string) {
  const invalidate = useQuestionInvalidation(wsId, issueId);
  const draftKey = questionDraftKey(wsId, questionId);
  return useMutation({
    mutationKey: questionMutationKeys.askBack(wsId, questionId),
    mutationFn: (data: AskBackQuestionRequest) => api.askBackQuestion(questionId, data),
    onSuccess: (_data, { text }) => useQuestionDraftStore.getState().clearSentAskBack(draftKey, text),
    onSettled: () => invalidate(questionId),
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
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: { id: string }) => api.wakeQuestionAgent(id),
    onSettled: (_data, _error, { id }) => Promise.all([
      invalidateIssueQuestions(qc, wsId, issueId, id),
      qc.invalidateQueries({ queryKey: issueKeys.timeline(issueId) }),
    ]),
  });
}
