// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import { TODO_KINDS } from "../types/todo";

function respond(body: unknown, status = 200) {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

const row = {
  id: "todo-1", workspace_id: "ws-1", kind: "question", title: "Review the export",
  created_at: "2026-10-15T10:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
  setSchemaLogger(noopLogger);
});

describe("to-do API contract", () => {
  const client = new ApiClient("https://api.example.test");

  it("keeps every kind and defaults missing optional fields without grouping issue rows", async () => {
    respond(TODO_KINDS.map((kind) => ({ ...row, id: kind, kind, issue_id: "same-issue" })));
    const result = await client.listTodos();
    expect(result).toHaveLength(TODO_KINDS.length);
    expect(result[0]).toMatchObject({
      issue_id: "same-issue", actor_type: null, overdue: false, details: {}, done_at: null,
    });
  });

  it("preserves new server enum values and extra details", async () => {
    respond([{ ...row, kind: "future_kind", issue_status: "future_status", question_answer_type: "future_answer",
      details: { prompt: "Create a report", retry: { task: "run-1" } }, extra_field: true }]);
    expect((await client.listTodos())[0]).toMatchObject({
      kind: "future_kind", issue_status: "future_status", question_answer_type: "future_answer",
      details: { retry: { task: "run-1" } }, extra_field: true,
    });
  });

  it.each([null, {}, [{ ...row, id: 42 }], [{ ...row, overdue: "yes" }]])(
    "logs malformed list responses and returns a safe empty list (%j)", async (body) => {
      const warn = vi.fn();
      setSchemaLogger({ ...noopLogger, warn });
      respond(body);
      await expect(client.listTodos()).resolves.toEqual([]);
      expect(warn).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({ endpoint: "GET /api/todos" }));
    },
  );

  it("reads account-level summary and rejects malformed counts", async () => {
    const fetch = respond([{ workspace_id: "ws-1", count: 3 }, { workspace_id: "ws-2", count: 2 }]);
    await expect(client.getTodoSummary()).resolves.toHaveLength(2);
    expect(fetch.mock.calls[0]?.[0]).toBe("https://api.example.test/api/todos/summary");
    respond([{ workspace_id: "ws-1", count: "3" }]);
    await expect(client.getTodoSummary()).resolves.toEqual([]);
  });

  it("posts Mark done and parses an already closed row idempotently", async () => {
    const fetch = respond({ ...row, done_at: "2026-10-15T11:00:00Z", done_by: "member" });
    await expect(client.markTodoDone("todo/1")).resolves.toMatchObject({ done_by: "member" });
    expect(fetch.mock.calls[0]?.[0]).toBe("https://api.example.test/api/todos/todo%2F1/done");
    expect(fetch.mock.calls[0]?.[1]).toMatchObject({ method: "POST" });
  });

  it("does not treat a malformed done response as a successful close", async () => {
    respond({ id: 42 });
    await expect(client.markTodoDone("todo-1")).rejects.toBeInstanceOf(Error);
  });

  it("surfaces server errors instead of hiding them behind an empty list", async () => {
    respond({ error: "not found" }, 404);
    await expect(client.markTodoDone("other-member-row")).rejects.toMatchObject({ status: 404 });
  });
});
