// @vitest-environment node
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { todoKeys, todoCountForWorkspace, hasOtherWorkspaceTodos, todoWorkspaceIds } from "./queries";
import { onTodoOpened, onTodoClosed, onTodosInvalidate } from "./ws-updaters";
import type { Todo } from "../types/todo";

const row: Todo = {
  id: "todo-1", workspace_id: "ws-1", kind: "question", title: "Review",
  issue_id: "issue-1", issue_identifier: "WORK-1", issue_title: "Review", issue_status: "in_review",
  question_id: "q1", question_answer_type: "approval", comment_id: null, task_id: null, autopilot_id: null,
  actor_type: "agent", actor_id: "agent-1", body: null, details: {}, due_date: null,
  overdue: false, created_at: "2026-10-15T10:00:00Z", done_at: null, done_by: null,
};

describe("to-do projections", () => {
  it("counts rows, not issues, and derives workspace dots from the account summary", () => {
    const summary = [{ workspace_id: "ws-1", count: 3 }, { workspace_id: "ws-2", count: 2 },
      { workspace_id: "empty", count: 0 }];
    expect(todoKeys.summary()).toEqual(["todos", "summary"]);
    expect(todoCountForWorkspace(summary, "missing")).toBe(0);
    expect(todoCountForWorkspace(summary, "ws-1")).toBe(3);
    expect(hasOtherWorkspaceTodos(summary, "ws-1")).toBe(true);
    expect(hasOtherWorkspaceTodos(summary.slice(0, 1), "ws-1")).toBe(false);
    expect(todoWorkspaceIds(summary)).toEqual(new Set(["ws-1", "ws-2"]));
  });

  it("invalidates only the opened row's workspace and the shared summary without inserting a hidden review", async () => {
    const qc = new QueryClient();
    qc.setQueryData(todoKeys.list("ws-1"), [row]);
    qc.setQueryData(todoKeys.list("ws-2"), []);
    qc.setQueryData(todoKeys.summary(), [{ workspace_id: "ws-1", count: 1 }]);
    await onTodoOpened(qc, { ...row, id: "review", kind: "review" });
    expect(qc.getQueryData(todoKeys.list("ws-1"))).toEqual([row]);
    expect(qc.getQueryState(todoKeys.list("ws-1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(todoKeys.list("ws-2"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(todoKeys.summary())?.isInvalidated).toBe(true);
    qc.clear();
  });

  it("closes just one row, preserving siblings on its issue and refreshing hidden reviews and summary", async () => {
    const qc = new QueryClient();
    const sibling = { ...row, id: "mention", kind: "mention" };
    qc.setQueryData(todoKeys.list("ws-1"), [row, sibling]);
    qc.setQueryData(todoKeys.list("ws-2"), [{ ...row, id: "other", workspace_id: "ws-2" }]);
    qc.setQueryData(todoKeys.summary(), [{ workspace_id: "ws-1", count: 2 }]);
    await onTodoClosed(qc, row.id);
    await onTodoClosed(qc, row.id);
    expect(qc.getQueryData(todoKeys.list("ws-1"))).toEqual([sibling]);
    expect(qc.getQueryData<Todo[]>(todoKeys.list("ws-2"))).toHaveLength(1);
    expect(qc.getQueryState(todoKeys.summary())?.isInvalidated).toBe(true);
    qc.clear();
  });

  it("cancels a pre-event first load so its stale response cannot win", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
    let resolveOld!: (rows: Todo[]) => void;
    let requests = 0;
    const observer = new QueryObserver(qc, {
      queryKey: todoKeys.list("ws-1"),
      queryFn: () => ++requests === 1 ? new Promise<Todo[]>((resolve) => { resolveOld = resolve; }) : Promise.resolve([row]),
    });
    const unsubscribe = observer.subscribe(() => {});
    await onTodosInvalidate(qc, "ws-1");
    resolveOld([]);
    await Promise.resolve();
    expect(requests).toBe(2);
    expect(qc.getQueryData(todoKeys.list("ws-1"))).toEqual([row]);
    unsubscribe();
    qc.clear();
  });
});
