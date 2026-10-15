import { useMutation, useQueryClient } from "@tanstack/react-query";
import { EMPTY_QUESTION_DRAFT, questionDraftKey, useQuestionDraftStore, type QuestionDraft } from "./draft-store";
import { api } from "../api";
import type {
  AnswerQuestionRequest,
  AskBackQuestionRequest,
  CancelQuestionRequest,
  ReassignQuestionRequest,
} from "../types/question";
import { invalidateQuestionRows, invalidateScopeQuestions } from "./ws-updaters";
import type { QuestionScope } from "./scope";
import { issueKeys } from "../issues/queries";

/**
 * Answers, ask-backs, cancels and reassignments also move the Questions
 * page's rows and count. `scope` is the issue or chat the card shows in.
 */
function useAnsweringInvalidation(wsId: string, scope: QuestionScope) {
  const qc = useQueryClient();
  return (id: string) => Promise.all([
    invalidateScopeQuestions(qc, wsId, scope, id),
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
  cancel: (wsId: string, questionId: string) => ["questions", wsId, "cancel", questionId] as const,
};

export function useAnswerQuestion(wsId: string, scope: QuestionScope, questionId: string) {
  const invalidate = useAnsweringInvalidation(wsId, scope);
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

export function useAskBackQuestion(wsId: string, scope: QuestionScope, questionId: string) {
  const invalidate = useAnsweringInvalidation(wsId, scope);
  const draftKey = questionDraftKey(wsId, questionId);
  return useMutation({
    mutationKey: questionMutationKeys.askBack(wsId, questionId),
    mutationFn: (data: AskBackQuestionRequest) => api.askBackQuestion(questionId, data),
    onSuccess: (_data, { text }) => useQuestionDraftStore.getState().clearSentAskBack(draftKey, text),
    onSettled: () => invalidate(questionId),
  });
}

/** Ends the question for every answerer, keeping the answers sent before it. */
export function useCancelQuestion(wsId: string, scope: QuestionScope, questionId: string) {
  const invalidate = useAnsweringInvalidation(wsId, scope);
  return useMutation({
    mutationKey: questionMutationKeys.cancel(wsId, questionId),
    mutationFn: (data: CancelQuestionRequest) => api.cancelQuestion(questionId, data),
    onSettled: () => invalidate(questionId),
  });
}

export function useReassignQuestion(wsId: string, scope: QuestionScope) {
  const invalidate = useAnsweringInvalidation(wsId, scope);
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & ReassignQuestionRequest) => api.reassignQuestion(id, data),
    onSettled: (_data, _error, { id }) => invalidate(id),
  });
}

export function useWakeQuestionAgent(wsId: string, scope: QuestionScope) {
  const qc = useQueryClient();
  return useMutation({
    // `target` is one of the question's wake targets; without it the asking
    // agent wakes, taken to wake in the card's own scope.
    mutationFn: ({ id, target }: { id: string; target?: { agentId: string; scope: QuestionScope } }) =>
      api.wakeQuestionAgent(id, target?.agentId),
    onSettled: (_data, _error, { id, target }) => {
      const woken = target?.scope ?? scope;
      return Promise.all([
        invalidateScopeQuestions(qc, wsId, scope, id),
        // A wake in a chat starts a turn there, which moves that chat's offers;
        // one on an issue writes to its timeline.
        "chatSessionId" in woken
          ? invalidateScopeQuestions(qc, wsId, woken, id)
          : qc.invalidateQueries({ queryKey: issueKeys.timeline(woken.issueId) }),
      ]);
    },
  });
}
