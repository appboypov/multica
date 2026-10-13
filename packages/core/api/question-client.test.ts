import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { EMPTY_QUESTION, EMPTY_QUESTION_WAKE_RESULT } from "./schemas";

const question = {
  id: "question-1", workspace_id: "ws-1", asker_agent_id: "pieter",
  asked_in_task_id: "run-3", prompt: "Per line or per total?",
  answer_type: "single_choice", settings: { options: ["per line", "per total"], allow_other: false },
  suggested_answer: { option: "per line" }, status: "open",
  created_at: "2026-10-12T20:00:00Z", updated_at: "2026-10-12T20:00:00Z",
};

function respond(body: unknown, status = 200) {
  const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => vi.unstubAllGlobals());

describe("question API boundary", () => {
  it("loads the issue list and question through their schemas", async () => {
    const fetchMock = respond([question]);
    const client = new ApiClient("https://api.example.test");
    expect(await client.listIssueQuestions("issue-1")).toMatchObject([{ id: "question-1", answers: [] }]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://api.example.test/api/issues/issue-1/questions");
    respond(question);
    expect(await client.getQuestion("question-1")).toMatchObject({ id: "question-1", wake_available: false });
  });

  it.each(["answer", "ask-back", "reassign"] as const)("posts %s without starting a run", async (action) => {
    const fetchMock = respond(question);
    const client = new ApiClient("https://api.example.test");
    const answer = { value: { option: "per line" }, note: "As before" };
    const askBack = { text: "Which invoices?" };
    const reassign = { user_id: "anouk", note: "You set the rules" };
    const body = action === "answer" ? answer : action === "ask-back" ? askBack : reassign;
    const result = action === "answer"
      ? await client.answerQuestion("question-1", answer)
      : action === "ask-back"
        ? await client.askBackQuestion("question-1", askBack)
        : await client.reassignQuestion("question-1", reassign);
    expect(result.id).toBe("question-1");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith(`https://api.example.test/api/questions/question-1/${action}`,
      expect.objectContaining({ method: "POST", body: JSON.stringify(body) }));
  });

  it("wakes only through the explicit endpoint and parses its 202", async () => {
    const fetchMock = respond({ task_id: "run-4", answer_count: 1 }, 202);
    expect(await new ApiClient("https://api.example.test").wakeQuestionAgent("question-1"))
      .toEqual({ task_id: "run-4", answer_count: 1 });
    expect(fetchMock).toHaveBeenCalledWith("https://api.example.test/api/questions/question-1/wake",
      expect.objectContaining({ method: "POST" }));
  });

  it("tolerates malformed responses from every question endpoint", async () => {
    respond(null);
    const client = new ApiClient("https://api.example.test");
    expect(await client.listIssueQuestions("issue-1")).toEqual([]);
    expect(await client.getQuestion("question-1")).toEqual(EMPTY_QUESTION);
    expect(await client.answerQuestion("question-1", { value: { option: "per line" } })).toEqual(EMPTY_QUESTION);
    expect(await client.askBackQuestion("question-1", { text: "Which invoices?" })).toEqual(EMPTY_QUESTION);
    expect(await client.reassignQuestion("question-1", { user_id: "anouk", note: "Your rules" })).toEqual(EMPTY_QUESTION);
    expect(await client.wakeQuestionAgent("question-1")).toEqual(EMPTY_QUESTION_WAKE_RESULT);
  });

  it("preserves the server's validation refusal", async () => {
    respond({ error: "at least 2 options are needed" }, 422);
    await expect(new ApiClient("https://api.example.test").answerQuestion("question-1", { value: { options: [] } }))
      .rejects.toMatchObject({ status: 422, message: "at least 2 options are needed" });
  });

  it("uploads answer attachments with question ownership and an abort signal", async () => {
    const fetchMock = respond({ id: "attachment-1", url: "/files/1" });
    const controller = new AbortController();
    const result = await new ApiClient("https://api.example.test").uploadFile(
      new File(["invoice"], "invoice.txt", { type: "text/plain" }),
      { questionId: "question-1" }, controller.signal,
    );
    expect(result.id).toBe("attachment-1");
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(init.signal).toBe(controller.signal);
    expect((init.body as FormData).get("question_id")).toBe("question-1");
    expect((init.body as FormData).get("issue_id")).toBeNull();
    respond(null);
    expect(await new ApiClient("https://api.example.test").uploadFile(new File([""], "empty.txt"), { questionId: "question-1" }))
      .toMatchObject({ id: "" });
  });
});
