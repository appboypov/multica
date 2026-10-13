// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { EMPTY_QUESTION } from "../api/schemas";
import type { Question } from "../types/question";
import { issueQuestionsOptions, questionDetailOptions, questionKeys } from "./queries";
import { handleQuestionEvent } from "./ws-updaters";
import { issueKeys } from "../issues/queries";

vi.mock("../api", () => ({ api: { listIssueQuestions: vi.fn(), getQuestion: vi.fn() } }));

function linkedQuestion(id: string, issueId: string): Question {
  return { ...EMPTY_QUESTION, id, links: [{ link_type: "issue", link_id: issueId, position: 0 }] };
}

let qc: QueryClient;
beforeEach(() => {
  qc = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
});
afterEach(() => { qc.clear(); vi.clearAllMocks(); });

describe("question query ownership", () => {
  it("includes workspace identity in both cache projections", () => {
    expect(questionKeys.byIssue("ws-1", "issue-1")).toEqual(["questions", "ws-1", "issue", "issue-1"]);
    expect(questionKeys.detail("ws-1", "q-1")).toEqual(["questions", "ws-1", "detail", "q-1"]);
    expect(questionKeys.byIssue("ws-1", "issue-1")).not.toEqual(questionKeys.byIssue("ws-2", "issue-1"));
    expect(questionKeys.detail("ws-1", "q-1")).not.toEqual(questionKeys.detail("ws-2", "q-1"));
  });

  it("fetches lists and details through the API into their scoped caches", async () => {
    const question = linkedQuestion("q-1", "issue-1");
    vi.mocked(api.listIssueQuestions).mockResolvedValue([question]);
    vi.mocked(api.getQuestion).mockResolvedValue(question);
    expect(await qc.fetchQuery(issueQuestionsOptions("ws-1", "issue-1"))).toEqual([question]);
    expect(await qc.fetchQuery(questionDetailOptions("ws-1", "q-1"))).toEqual(question);
    expect(api.listIssueQuestions).toHaveBeenCalledWith("issue-1");
    expect(api.getQuestion).toHaveBeenCalledWith("q-1");
  });

  it("does not enable queries until workspace and resource identities exist", () => {
    expect(issueQuestionsOptions("", "issue-1").enabled).toBe(false);
    expect(issueQuestionsOptions("ws-1", "").enabled).toBe(false);
    expect(questionDetailOptions("", "q-1").enabled).toBe(false);
    expect(questionDetailOptions("ws-1", "").enabled).toBe(false);
  });
});

describe("question WebSocket invalidation", () => {
  function seed() {
    qc.setQueryData(questionKeys.byIssue("ws-1", "issue-1"), [linkedQuestion("q-1", "issue-1")]);
    qc.setQueryData(questionKeys.detail("ws-1", "q-1"), linkedQuestion("q-1", "issue-1"));
    qc.setQueryData(questionKeys.detail("ws-1", "q-2"), linkedQuestion("q-2", "issue-1"));
    qc.setQueryData(questionKeys.byIssue("ws-1", "issue-2"), [linkedQuestion("q-other", "issue-2")]);
    qc.setQueryData(questionKeys.detail("ws-1", "q-other"), linkedQuestion("q-other", "issue-2"));
    qc.setQueryData(questionKeys.byIssue("ws-2", "issue-1"), []);
    qc.setQueryData(questionKeys.detail("ws-2", "q-1"), linkedQuestion("q-1", "issue-1"));
    qc.setQueryData(issueKeys.timeline("issue-1"), []);
    qc.setQueryData(issueKeys.timeline("issue-2"), []);
  }

  it.each([
    "question:created", "question:updated", "task:queued", "task:dispatch", "task:running",
    "task:waiting_local_directory", "task:progress", "task:completed", "task:failed", "task:cancelled", "task:message",
  ])("%s refreshes the issue and its cards without crossing workspace or issue boundaries", async (type) => {
    seed();
    await handleQuestionEvent(qc, "ws-1", type, { issue_id: "issue-1", question_id: "q-1" });
    expect(qc.getQueryState(questionKeys.byIssue("ws-1", "issue-1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(questionKeys.detail("ws-1", "q-1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(questionKeys.detail("ws-1", "q-2"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(questionKeys.byIssue("ws-1", "issue-2"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(questionKeys.detail("ws-1", "q-other"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(questionKeys.byIssue("ws-2", "issue-1"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(questionKeys.detail("ws-2", "q-1"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(issueKeys.timeline("issue-1"))?.isInvalidated).toBe(type !== "question:created");
    expect(qc.getQueryState(issueKeys.timeline("issue-2"))?.isInvalidated).toBe(false);
  });

  it("invalidates a named question even before its cached links include the issue", async () => {
    qc.setQueryData(questionKeys.detail("ws-1", "q-1"), { ...EMPTY_QUESTION, id: "q-1" });
    await handleQuestionEvent(qc, "ws-1", "question:updated", { issue_id: "issue-1", question_id: "q-1" });
    expect(qc.getQueryState(questionKeys.detail("ws-1", "q-1"))?.isInvalidated).toBe(true);
  });

  it.each([null, {}, { issue_id: "" }, { issue_id: 42 }, { chat_session_id: "chat-1" }])("ignores a malformed or non-issue payload %j", async (payload) => {
    seed();
    await handleQuestionEvent(qc, "ws-1", "task:completed", payload);
    expect(qc.getQueryState(questionKeys.byIssue("ws-1", "issue-1"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(issueKeys.timeline("issue-1"))?.isInvalidated).toBe(false);
  });

  it("ignores unrelated events and missing workspace context", async () => {
    seed();
    await handleQuestionEvent(qc, "ws-1", "comment:created", { issue_id: "issue-1" });
    await handleQuestionEvent(qc, "", "question:created", { issue_id: "issue-1" });
    expect(qc.getQueryState(questionKeys.byIssue("ws-1", "issue-1"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(issueKeys.timeline("issue-1"))?.isInvalidated).toBe(false);
  });
});
