/** @vitest-environment jsdom */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import type { Todo, TodoWorkspaceCount } from "../types/todo";
import { useMarkTodoDone } from "./mutations";
import { todoKeys, todoCountForWorkspace } from "./queries";

vi.mock("../hooks", () => ({ useWorkspaceId: () => "ws-1" }));
afterEach(cleanup);

const row: Todo = {
  id: "todo-1", workspace_id: "ws-1", kind: "mention", title: "Review",
  issue_id: "issue-1", issue_identifier: "WORK-1", issue_title: "Review", issue_status: "todo",
  question_id: null, question_answer_type: null, comment_id: "comment-1", task_id: null, autopilot_id: null,
  actor_type: "agent", actor_id: "agent-1", body: null, details: {}, due_date: null,
  overdue: false, created_at: "2026-10-15T10:00:00Z", done_at: null, done_by: null,
};

function setup(markTodoDone: (id: string) => Promise<Todo>) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  qc.setQueryData(todoKeys.list("ws-1"), [row, { ...row, id: "sibling" }]);
  qc.setQueryData(todoKeys.summary(), [{ workspace_id: "ws-1", count: 2 }, { workspace_id: "ws-2", count: 4 }]);
  setApiInstance({ markTodoDone } as unknown as ApiClient);
  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  }
  const hook = renderHook(() => useMarkTodoDone(), { wrapper: Wrapper });
  return { qc, ...hook };
}

function count(qc: QueryClient) {
  return todoCountForWorkspace(qc.getQueryData<TodoWorkspaceCount[]>(todoKeys.summary()) ?? [], "ws-1");
}

describe("Mark done", () => {
  it("immediately removes only the selected row and decrements its workspace number", async () => {
    let resolve!: (todo: Todo) => void;
    const { qc, result } = setup(vi.fn(() => new Promise<Todo>((done) => { resolve = done; })));
    act(() => result.current.mutate(row.id));
    await waitFor(() => expect(qc.getQueryData<Todo[]>(todoKeys.list("ws-1"))?.map((todo) => todo.id)).toEqual(["sibling"]));
    expect(count(qc)).toBe(1);
    expect(qc.getQueryData<TodoWorkspaceCount[]>(todoKeys.summary())?.[1]?.count).toBe(4);
    await act(async () => resolve({ ...row, done_at: "2026-10-15T11:00:00Z", done_by: "member" }));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(qc.getQueryState(todoKeys.summary())?.isInvalidated).toBe(true);
    qc.clear();
  });

  it("restores the row and number when the server rejects the close", async () => {
    let reject!: (error: Error) => void;
    const { qc, result } = setup(vi.fn(() => new Promise<Todo>((_resolve, fail) => { reject = fail; })));
    act(() => result.current.mutate(row.id));
    await waitFor(() => expect(count(qc)).toBe(1));
    await act(async () => reject(new Error("offline")));
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(qc.getQueryData<Todo[]>(todoKeys.list("ws-1"))).toContainEqual(row);
    expect(count(qc)).toBe(2);
    qc.clear();
  });

  it("does not decrement twice on repeated clicks or resurrect a different completed row on rollback", async () => {
    const pending: { resolve: (todo: Todo) => void; reject: (error: Error) => void }[] = [];
    const { qc, result } = setup(vi.fn(() => new Promise<Todo>((resolve, reject) => { pending.push({ resolve, reject }); })));
    act(() => result.current.mutate(row.id));
    await waitFor(() => expect(count(qc)).toBe(1));
    act(() => result.current.mutate(row.id));
    await waitFor(() => expect(pending).toHaveLength(2));
    expect(count(qc)).toBe(1);
    act(() => result.current.mutate("sibling"));
    await waitFor(() => expect(count(qc)).toBe(0));
    await act(async () => pending[0]!.reject(new Error("failed")));
    expect(qc.getQueryData<Todo[]>(todoKeys.list("ws-1"))?.map((todo) => todo.id)).toEqual([row.id]);
    await act(async () => {
      pending[1]!.resolve({ ...row, done_by: "member" });
      pending[2]!.resolve({ ...row, id: "sibling", done_by: "member" });
    });
    qc.clear();
  });
});
