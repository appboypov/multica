import { useMutation, useQueryClient } from "@tanstack/react-query";
import { EMPTY_QUESTION_DRAFT, questionDraftKey, useQuestionDraftStore, type QuestionDraft } from "./draft-store";
import { api } from "../api";
import type { AnswerQuestionRequest, AskBackQuestionRequest, ReassignQuestionRequest } from "../types/question";
import { invalidateIssueQuestions, invalidateQuestionRows } from "./ws-updaters";
import { issueKeys } from "../issues/queries";

/** Answers, ask-backs and reassignments also move the Questions page's rows and count. */
function useAnsweringInvalidation(wsId: string, issueId: string | null) {
  const qc = useQueryClient();
  return (id: string) => Promise.all([
    invalidateIssueQuestions(qc, wsId, issueId, id),
    invalidateQuestionRows(qc, wsId),
  ]);
}

/**
 * Mutation keys per question, so every card showing the question sees its
 * send as pending, also a card mounted again after the send started.
 */
export const questionMutationKeys = {
  answer: (wsId: string, questionId: string) => ["questions", wsId, "answer", questionId] as const,
  askBack: (wsId: string, questionId: string) => ["questions", wsId, "ask-back", questionId] as const,
};

export function useAnswerQuestion(wsId: string, issueId: string | null, questionId: string) {
  const invalidate = useAnsweringInvalidation(wsId, issueId);
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

export function useAskBackQuestion(wsId: string, issueId: string | null, questionId: string) {
  const invalidate = useAnsweringInvalidation(wsId, issueId);
  const draftKey = questionDraftKey(wsId, questionId);
  return useMutation({
    mutationKey: questionMutationKeys.askBack(wsId, questionId),
    mutationFn: (data: AskBackQuestionRequest) => api.askBackQuestion(questionId, data),
    onSuccess: (_data, { text }) => useQuestionDraftStore.getState().clearSentAskBack(draftKey, text),
    onSettled: () => invalidate(questionId),
  });
}

export function useReassignQuestion(wsId: string, issueId: string | null) {
  const invalidate = useAnsweringInvalidation(wsId, issueId);
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & ReassignQuestionRequest) => api.reassignQuestion(id, data),
    onSettled: (_data, _error, { id }) => invalidate(id),
  });
}

export function useWakeQuestionAgent(wsId: string, issueId: string | null) {
  const qc = useQueryClient();
  return useMutation({
    // `target` is one of the question's wake targets; without it the asking agent wakes on this issue.
    mutationFn: ({ id, target }: { id: string; target?: { agentId: string; issueId: string } }) =>
      api.wakeQuestionAgent(id, target?.agentId),
    onSettled: (_data, _error, { id, target }) => {
      const timelineIssueId = target?.issueId ?? issueId;
      return Promise.all([
        invalidateIssueQuestions(qc, wsId, issueId, id),
        timelineIssueId && qc.invalidateQueries({ queryKey: issueKeys.timeline(timelineIssueId) }),
      ]);
    },
  });
}
