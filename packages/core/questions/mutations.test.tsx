// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useIsMutating } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { EMPTY_QUESTION } from "../api/schemas";
import { questionKeys } from "./queries";
import { questionMutationKeys, useAnswerQuestion, useAskBackQuestion, useReassignQuestion, useWakeQuestionAgent } from "./mutations";
import { issueKeys } from "../issues/queries";
import { EMPTY_QUESTION_DRAFT, questionDraftKey, useQuestionDraftStore } from "./draft-store";

const transport = vi.hoisted(() => ({
  answerQuestion: vi.fn(), askBackQuestion: vi.fn(), reassignQuestion: vi.fn(), wakeQuestionAgent: vi.fn(), uploadFile: vi.fn(),
}));
vi.mock("../api", () => ({ api: transport }));
afterEach(() => vi.resetAllMocks());

const actions = [
  { name: "answer", method: "answerQuestion", useSubmit() {
    const mutation = useAnswerQuestion("ws-1", "issue-1", "q-1");
    return () => mutation.mutateAsync({ value: { option: "per line" }, note: "As before" });
  } },
  { name: "ask back", method: "askBackQuestion", useSubmit() {
    const mutation = useAskBackQuestion("ws-1", "issue-1", "q-1");
    return () => mutation.mutateAsync({ text: "Which invoices?" });
  } },
  { name: "reassign", method: "reassignQuestion", useSubmit() {
    const mutation = useReassignQuestion("ws-1", "issue-1");
    return () => mutation.mutateAsync({ id: "q-1", user_id: "anouk", note: "Your rules" });
  } },
  { name: "Wake", method: "wakeQuestionAgent", useSubmit() {
    const mutation = useWakeQuestionAgent("ws-1", "issue-1");
    return () => mutation.mutateAsync({ id: "q-1" });
  } },
] as const;

describe.each(actions)("$name mutation", ({ method, useSubmit }) => {
  it.each([false, true])("invalidates both projections on settle (failure=%s)", async (fails) => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    const question = { ...EMPTY_QUESTION, id: "q-1", links: [{ link_type: "issue" as const, link_id: "issue-1", position: 0 }] };
    qc.setQueryData(questionKeys.byIssue("ws-1", "issue-1"), [question]);
    qc.setQueryData(questionKeys.detail("ws-1", "q-1"), question);
    qc.setQueryData(questionKeys.detail("ws-1", "q-2"), { ...question, id: "q-2" });
    qc.setQueryData(questionKeys.byIssue("ws-2", "issue-1"), []);
    qc.setQueryData(issueKeys.timeline("issue-1"), []);
    qc.setQueryData(issueKeys.timeline("issue-2"), []);
    const failure = new Error("Server refused the action");
    if (fails) transport[method].mockRejectedValue(failure);
    else transport[method].mockResolvedValue(question);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result, unmount } = renderHook<() => Promise<unknown>, unknown>(useSubmit, { wrapper });
    try {
      await act(async () => {
        if (fails) await expect(result.current()).rejects.toBe(failure);
        else await result.current();
      });
      expect(transport[method]).toHaveBeenCalledTimes(1);
      if (method !== "wakeQuestionAgent") expect(transport.wakeQuestionAgent).not.toHaveBeenCalled();
      expect(qc.getQueryState(questionKeys.byIssue("ws-1", "issue-1"))?.isInvalidated).toBe(true);
      expect(qc.getQueryState(questionKeys.detail("ws-1", "q-1"))?.isInvalidated).toBe(true);
      expect(qc.getQueryState(questionKeys.detail("ws-1", "q-2"))?.isInvalidated).toBe(true);
      expect(qc.getQueryState(questionKeys.byIssue("ws-2", "issue-1"))?.isInvalidated).toBe(false);
      expect(qc.getQueryState(issueKeys.timeline("issue-1"))?.isInvalidated).toBe(method === "wakeQuestionAgent");
      expect(qc.getQueryState(issueKeys.timeline("issue-2"))?.isInvalidated).toBe(false);
    } finally { unmount(); qc.clear(); }
  });
});

describe("question drafts after a send", () => {
  const key = questionDraftKey("ws-1", "q-1");
  const unsent = { answer: { option: "per line" }, note: "As before", noteOpen: true, askBackOpen: true, askBackText: "Which invoices?", editing: true };

  it.each([
    { name: "answer", method: "answerQuestion", left: undefined, useSubmit: () => {
      const mutation = useAnswerQuestion("ws-1", "issue-1", "q-1");
      return () => mutation.mutate({ value: { option: "per line" } });
    } },
    { name: "ask back", method: "askBackQuestion", left: { ...unsent, askBackOpen: false, askBackText: "" }, useSubmit: () => {
      const mutation = useAskBackQuestion("ws-1", "issue-1", "q-1");
      return () => mutation.mutate({ text: "Which invoices?" });
    } },
  ] as const)("clears what the $name sent even when its card unmounted before the reply", async ({ method, left, useSubmit }) => {
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    useQuestionDraftStore.setState({ drafts: { [key]: { ...EMPTY_QUESTION_DRAFT, ...unsent, updatedAt: Date.now() } } });
    const reply = Promise.withResolvers<unknown>();
    transport[method].mockReturnValue(reply.promise);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result, unmount } = renderHook<() => void, unknown>(useSubmit, { wrapper });
    try {
      act(() => result.current());
      unmount();
      reply.resolve({ ...EMPTY_QUESTION, id: "q-1" });
      await waitFor(() => expect(transport[method]).toHaveBeenCalledTimes(1));
      await waitFor(() => {
        const draft = useQuestionDraftStore.getState().drafts[key];
        if (left === undefined) expect(draft).toBeUndefined();
        else expect(draft).toMatchObject(left);
      });
    } finally { useQuestionDraftStore.setState({ drafts: {} }); qc.clear(); }
  });

  it.each([
    { name: "answer", method: "answerQuestion", rewrite: { answer: { option: "on the total" } }, useSubmit: () => {
      const mutation = useAnswerQuestion("ws-1", "issue-1", "q-1");
      return () => mutation.mutate({ value: { option: "per line" }, note: "As before" });
    } },
    { name: "ask back", method: "askBackQuestion", rewrite: { askBackText: "Which invoices, and since when?" }, useSubmit: () => {
      const mutation = useAskBackQuestion("ws-1", "issue-1", "q-1");
      return () => mutation.mutate({ text: "Which invoices?" });
    } },
  ] as const)("keeps what was written after the $name was sent", async ({ method, rewrite, useSubmit }) => {
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    useQuestionDraftStore.setState({ drafts: { [key]: { ...EMPTY_QUESTION_DRAFT, ...unsent, updatedAt: Date.now() } } });
    const reply = Promise.withResolvers<unknown>();
    transport[method].mockReturnValue(reply.promise);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result, unmount } = renderHook<() => void, unknown>(useSubmit, { wrapper });
    try {
      act(() => result.current());
      await waitFor(() => expect(transport[method]).toHaveBeenCalledTimes(1));
      unmount();
      // Written on a card mounted again while the send was on its way.
      useQuestionDraftStore.getState().setDraft(key, rewrite);
      await act(async () => reply.resolve({ ...EMPTY_QUESTION, id: "q-1" }));
      await waitFor(() => expect(qc.isMutating()).toBe(0));
      expect(useQuestionDraftStore.getState().drafts[key]).toMatchObject(rewrite);
    } finally { useQuestionDraftStore.setState({ drafts: {} }); qc.clear(); }
  });

  it("shows a send as pending to a card mounted after it started, for that question only", async () => {
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const reply = Promise.withResolvers<unknown>();
    transport.answerQuestion.mockReturnValue(reply.promise);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const sender = renderHook(() => useAnswerQuestion("ws-1", "issue-1", "q-1"), { wrapper });
    try {
      act(() => sender.result.current.mutate({ value: { option: "per line" } }));
      sender.unmount();
      const pending = renderHook(() => ({
        same: useIsMutating({ mutationKey: questionMutationKeys.answer("ws-1", "q-1") }),
        other: useIsMutating({ mutationKey: questionMutationKeys.answer("ws-1", "q-2") }),
      }), { wrapper });
      expect(pending.result.current).toEqual({ same: 1, other: 0 });
      await act(async () => reply.resolve({ ...EMPTY_QUESTION, id: "q-1" }));
      await waitFor(() => expect(pending.result.current.same).toBe(0));
      pending.unmount();
    } finally { qc.clear(); }
  });
});
