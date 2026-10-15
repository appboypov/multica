import { afterEach, describe, expect, it } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { EMPTY_QUESTION } from "../api/schemas";
import { issueKeys } from "../issues/queries";
import type { Question } from "../types/question";
import { questionKeys } from "./queries";
import { handleQuestionEvent } from "./ws-updaters";

function chatQuestion(id: string, chatSessionId: string): Question {
  return {
    ...EMPTY_QUESTION,
    id,
    links: [{ link_type: "chat", link_id: chatSessionId, position: 0, member_id: "user-brian" }],
  };
}

describe("handleQuestionEvent", () => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  afterEach(() => qc.clear());

  function seed() {
    qc.setQueryData(questionKeys.byChat("ws-1", "chat-1"), [chatQuestion("q-1", "chat-1")]);
    qc.setQueryData(questionKeys.byChat("ws-1", "chat-2"), [chatQuestion("q-2", "chat-2")]);
    qc.setQueryData(questionKeys.byIssue("ws-1", "issue-412"), [
      { ...EMPTY_QUESTION, id: "q-3", links: [{ link_type: "issue", link_id: "issue-412", position: 0, member_id: null }] },
    ]);
    qc.setQueryData(questionKeys.byChat("ws-2", "chat-1"), []);
    qc.setQueryData(questionKeys.detail("ws-1", "q-1"), chatQuestion("q-1", "chat-1"));
    qc.setQueryData(questionKeys.detail("ws-1", "q-2"), chatQuestion("q-2", "chat-2"));
    qc.setQueryData(questionKeys.openCount("ws-1"), { count: 1, blocking_count: 0 });
    qc.setQueryData(issueKeys.timeline("issue-412"), []);
  }
  const invalidated = (key: readonly unknown[]) => qc.getQueryState(key)?.isInvalidated;

  it("invalidates the chat's questions on a question:updated with only chat_session_id", async () => {
    seed();
    await handleQuestionEvent(qc, "ws-1", "question:updated", {
      question_id: "q-1",
      issue_id: "",
      chat_session_id: "chat-1",
      reason: "cancelled",
    });
    expect(invalidated(questionKeys.byChat("ws-1", "chat-1"))).toBe(true);
    expect(invalidated(questionKeys.detail("ws-1", "q-1"))).toBe(true);
    expect(invalidated(questionKeys.openCount("ws-1"))).toBe(true);
    expect(invalidated(questionKeys.byChat("ws-1", "chat-2"))).toBe(false);
    expect(invalidated(questionKeys.detail("ws-1", "q-2"))).toBe(false);
    expect(invalidated(questionKeys.byChat("ws-2", "chat-1"))).toBe(false);
    expect(invalidated(questionKeys.byIssue("ws-1", "issue-412"))).toBe(false);
    expect(invalidated(issueKeys.timeline("issue-412"))).toBe(false);
  });

  it("invalidates both the chat and the issue of a question asked in a chat on an issue", async () => {
    seed();
    await handleQuestionEvent(qc, "ws-1", "question:updated", {
      question_id: "q-1",
      issue_id: "issue-412",
      chat_session_id: "chat-1",
      reason: "answered",
    });
    expect(invalidated(questionKeys.byChat("ws-1", "chat-1"))).toBe(true);
    expect(invalidated(questionKeys.byIssue("ws-1", "issue-412"))).toBe(true);
    expect(invalidated(issueKeys.timeline("issue-412"))).toBe(true);
    expect(invalidated(questionKeys.byChat("ws-1", "chat-2"))).toBe(false);
  });

  it("refreshes a chat's offers on its turn's task events, without touching the Questions page", async () => {
    seed();
    await handleQuestionEvent(qc, "ws-1", "task:completed", { task_id: "task-9", chat_session_id: "chat-1" });
    expect(invalidated(questionKeys.byChat("ws-1", "chat-1"))).toBe(true);
    expect(invalidated(questionKeys.openCount("ws-1"))).toBe(false);
  });

  it("ignores an event with neither an issue nor a chat", async () => {
    seed();
    await handleQuestionEvent(qc, "ws-1", "question:updated", { question_id: "q-1", issue_id: "" });
    expect(invalidated(questionKeys.byChat("ws-1", "chat-1"))).toBe(false);
    expect(invalidated(questionKeys.byIssue("ws-1", "issue-412"))).toBe(false);
  });
});
