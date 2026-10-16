// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

function respond(body: unknown, status = 200) {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}
const client = new ApiClient("https://api.example.test");
afterEach(() => vi.unstubAllGlobals());

describe("check API", () => {
  it("sends feedback and validates the answered to-do", async () => {
    const fetch = respond({ id: "check-1", workspace_id: "ws-1", kind: "check", title: "Check totals", created_at: "2026-10-17T10:00:00Z", details: { answer: "feedback", note: "Round totals" } });
    await expect(client.answerCheckTodo("check/1", "feedback", "Round totals")).resolves.toMatchObject({ details: { answer: "feedback" } });
    expect(fetch.mock.calls[0]?.[0]).toBe("https://api.example.test/api/todos/check%2F1/answer");
    expect(fetch.mock.calls[0]?.[1]).toMatchObject({ method: "POST", body: JSON.stringify({ answer: "feedback", note: "Round totals" }) });
  });
  it("rejects malformed answer and trust responses", async () => {
    respond({ id: 7 });
    await expect(client.answerCheckTodo("check-1", "pass", "")).rejects.toThrow("Invalid check answer response");
    respond([{ project_id: "p", passed: "five" }]);
    await expect(client.getProjectCheckTrust("p")).rejects.toThrow("Invalid project check trust response");
  });
  it("reads project trust, defaults optional fields and keeps unknown kinds", async () => {
    const fetch = respond([{ project_id: "p", kind: "future_kind", member_id: "brian", passed: 5, feedback: 0, streak: 5, automatic: true }]);
    await expect(client.getProjectCheckTrust("p/1")).resolves.toEqual([{ project_id: "p", kind: "future_kind", member_id: "brian", passed: 5, feedback: 0, streak: 5, automatic: true, automatic_passed: 0, automatic_opens: 0, proposal_id: null }]);
    expect(fetch.mock.calls[0]?.[0]).toBe("https://api.example.test/api/projects/p%2F1/check-trust");
  });
  it("surfaces server answer failures", async () => {
    respond({ error: "not found" }, 404);
    await expect(client.answerCheckTodo("check-1", "pass", "")).rejects.toMatchObject({ status: 404 });
  });
});
