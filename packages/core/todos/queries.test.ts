/** @vitest-environment jsdom */
import { QueryObserver } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { createQueryClient } from "../query-client";
import type { Todo } from "../types/todo";
import { todoListOptions } from "./queries";

const base: Todo = {
  id: "todo", workspace_id: "ws-1", kind: "assigned", title: "Customer import",
  issue_id: "issue-1", issue_identifier: "WORK-427", issue_title: "Customer import", issue_status: "todo",
  question_id: null, question_answer_type: null, comment_id: null, task_id: null, autopilot_id: null,
  actor_type: "member", actor_id: "member-mark", body: null, details: {}, due_date: null,
  overdue: false, created_at: "2026-10-16T08:00:00Z", done_at: null, done_by: null,
};

/** The server's list: overdue against its date at fetch time, overdue rows first, then newest. */
function serverList(rows: Todo[]): Todo[] {
  const today = new Date().toISOString().slice(0, 10);
  return rows
    .map((row) => ({ ...row, overdue: row.due_date !== null && row.due_date < today }))
    .sort((a, b) => Number(b.overdue) - Number(a.overdue) || b.created_at.localeCompare(a.created_at));
}

afterEach(() => {
  vi.useRealTimers();
});

describe("todoListOptions", () => {
  it("shows a row that became overdue past the date change first, without an event", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-16T23:58:00Z"));
    const dueToday = { ...base, id: "due-today", due_date: "2026-10-16", created_at: "2026-10-16T08:00:00Z" };
    const newer = { ...base, id: "newer", created_at: "2026-10-16T12:00:00Z" };
    setApiInstance({ listTodos: async () => serverList([dueToday, newer]) } as unknown as ApiClient);
    const qc = createQueryClient();
    const observer = new QueryObserver(qc, todoListOptions("ws-1"));
    const unsubscribe = observer.subscribe(() => {});

    await vi.advanceTimersByTimeAsync(0);
    expect(observer.getCurrentResult().data?.map((row) => [row.id, row.overdue])).toEqual([["newer", false], ["due-today", false]]);

    await vi.advanceTimersByTimeAsync(10 * 60 * 1000);
    expect(observer.getCurrentResult().data?.map((row) => [row.id, row.overdue])).toEqual([["due-today", true], ["newer", false]]);
    unsubscribe();
    qc.clear();
  });
});
