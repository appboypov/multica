// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { EMPTY_QUESTION } from "../api/schemas";
import { questionKeys } from "./queries";
import { useAnswerQuestion, useAskBackQuestion, useReassignQuestion, useWakeQuestionAgent, useUploadQuestionFile } from "./mutations";
import { issueKeys } from "../issues/queries";

const transport = vi.hoisted(() => ({
  answerQuestion: vi.fn(), askBackQuestion: vi.fn(), reassignQuestion: vi.fn(), wakeQuestionAgent: vi.fn(), uploadFile: vi.fn(),
}));
vi.mock("../api", () => ({ api: transport }));
afterEach(() => vi.resetAllMocks());

const actions = [
  { name: "answer", method: "answerQuestion", useSubmit() {
    const mutation = useAnswerQuestion("ws-1", "issue-1");
    return () => mutation.mutateAsync({ id: "q-1", value: { option: "per line" }, note: "As before" });
  } },
  { name: "ask back", method: "askBackQuestion", useSubmit() {
    const mutation = useAskBackQuestion("ws-1", "issue-1");
    return () => mutation.mutateAsync({ id: "q-1", text: "Which invoices?" });
  } },
  { name: "reassign", method: "reassignQuestion", useSubmit() {
    const mutation = useReassignQuestion("ws-1", "issue-1");
    return () => mutation.mutateAsync({ id: "q-1", user_id: "anouk", note: "Your rules" });
  } },
  { name: "Wake", method: "wakeQuestionAgent", useSubmit() {
    const mutation = useWakeQuestionAgent("ws-1", "issue-1");
    return () => mutation.mutateAsync({ id: "q-1" });
  } },
  { name: "upload", method: "uploadFile", useSubmit() {
    const mutation = useUploadQuestionFile("ws-1", "issue-1");
    return () => mutation.mutateAsync({ id: "q-1", file: new File(["invoice"], "invoice.txt") });
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
