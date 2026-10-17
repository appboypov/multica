/** @vitest-environment jsdom */
import { QueryClientProvider, useQuery } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import type { WSClient } from "../api/ws-client";
import { todoKeys, todoListOptions, todoSummaryOptions } from "../todos/queries";
import { workspaceKeys } from "../workspace/queries";
import { notificationPreferenceKeys } from "../notification-preferences/queries";
import { createQueryClient } from "../query-client";
import type { Todo } from "../types/todo";
import { handleTodoOpened, useRealtimeSync, type RealtimeSyncStores } from "./use-realtime-sync";

vi.mock("../platform/workspace-storage", () => ({
  getCurrentWsId: () => "ws-active",
  getCurrentSlug: () => "active",
  createWorkspaceAwareStorage: (adapter: unknown) => adapter,
  registerForWorkspaceRehydration: () => {},
}));
vi.mock("../paths", () => ({ useHasOnboarded: () => true, resolvePostAuthDestination: () => "/" }));

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  delete (globalThis as Record<string, unknown>).desktopAPI;
});

const todo: Todo = {
  id: "todo-1", workspace_id: "ws-source", kind: "question", title: "Approve the export",
  issue_id: "issue-1", issue_identifier: "WORK-1", issue_title: "Approve the export", issue_status: "in_review",
  question_id: "q1", question_answer_type: "approval", comment_id: null, task_id: null, autopilot_id: null,
  actor_type: "agent", actor_id: "agent-1", body: null, details: {}, due_date: null,
  overdue: false, created_at: "2026-10-15T10:00:00Z", done_at: null, done_by: null,
};

describe("todo realtime", () => {
  it("routes native banners to the source workspace and the to-do id, not the issue or active workspace", async () => {
    const qc = createQueryClient();
    qc.setQueryData(workspaceKeys.list(), [{ id: "ws-source", slug: "source" }, { id: "ws-active", slug: "active" }]);
    qc.setQueryData(notificationPreferenceKeys.all("ws-source"), { preferences: { system_notifications: "all" } });
    const showNotification = vi.fn();
    (globalThis as Record<string, unknown>).desktopAPI = { showNotification };
    vi.spyOn(document, "hasFocus").mockReturnValue(false);
    await handleTodoOpened(qc, todo);
    expect(showNotification).toHaveBeenCalledWith({
      slug: "source", itemId: todo.id, issueKey: todo.id, title: todo.title, body: "", destination: "inbox",
    });
    qc.clear();
  });

  it("honors focus and source-workspace mute without suppressing cache invalidation", async () => {
    const qc = createQueryClient();
    qc.setQueryData(workspaceKeys.list(), [{ id: "ws-source", slug: "source" }]);
    qc.setQueryData(notificationPreferenceKeys.all("ws-source"), { preferences: { system_notifications: "muted" } });
    qc.setQueryData(todoKeys.summary(), []);
    const showNotification = vi.fn();
    (globalThis as Record<string, unknown>).desktopAPI = { showNotification };
    const focus = vi.spyOn(document, "hasFocus").mockReturnValue(true);
    await handleTodoOpened(qc, todo);
    focus.mockReturnValue(false);
    await handleTodoOpened(qc, todo);
    expect(showNotification).not.toHaveBeenCalled();
    await waitFor(() => expect(qc.getQueryState(todoKeys.summary())?.isInvalidated).toBe(true));
    qc.clear();
  });

  it("uses source slug for a cold preference lookup", async () => {
    const qc = createQueryClient();
    qc.setQueryData(workspaceKeys.list(), [{ id: "ws-source", slug: "source" }]);
    const getNotificationPreferences = vi.fn(async () => ({ preferences: { system_notifications: "muted" } }));
    setApiInstance({ getNotificationPreferences } as unknown as ApiClient);
    const showNotification = vi.fn();
    (globalThis as Record<string, unknown>).desktopAPI = { showNotification };
    vi.spyOn(document, "hasFocus").mockReturnValue(false);
    await handleTodoOpened(qc, todo);
    expect(getNotificationPreferences).toHaveBeenCalledWith("source");
    expect(showNotification).not.toHaveBeenCalled();
    qc.clear();
  });

  it("wires opened/closed events, recovers on reconnect and unregisters subscriptions", async () => {
    const qc = createQueryClient();
    let rows: Todo[] = [];
    const listTodos = vi.fn(async () => rows);
    setApiInstance({
      listTodos,
      getTodoSummary: async () => rows.length ? [{ workspace_id: "ws-active", count: rows.length }] : [],
    } as unknown as ApiClient);
    const handlers: Record<string, (payload: unknown) => void | Promise<void>> = {};
    let reconnect = () => {};
    const ws = {
      on: (event: string, handler: (payload: unknown) => void | Promise<void>) => {
        handlers[event] = handler;
        return () => { delete handlers[event]; };
      },
      onAny: () => () => {},
      onReconnect: (handler: () => void) => { reconnect = handler; return () => {}; },
    } as unknown as WSClient;
    const stores = { authStore: { getState: () => ({ user: { id: "member-1" } }) } } as unknown as RealtimeSyncStores;
    function Wrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    }
    vi.spyOn(document, "hasFocus").mockReturnValue(true);
    const shell = renderHook(() => {
      useRealtimeSync(ws, stores);
      return { list: useQuery(todoListOptions("ws-active")), summary: useQuery(todoSummaryOptions()) };
    }, { wrapper: Wrapper });
    await waitFor(() => expect(shell.result.current.list.data).toEqual([]));
    rows = [{ ...todo, workspace_id: "ws-active" }];
    await act(async () => { await handlers["todo:opened"]!({ member_id: "member-1", todo: rows[0] }); });
    await waitFor(() => expect(shell.result.current.list.data).toEqual(rows));
    await waitFor(() => expect(shell.result.current.summary.data?.[0]?.count).toBe(1));
    rows = [];
    await act(async () => { await handlers["todo:closed"]!({ member_id: "member-1", todo_id: todo.id, kind: "question", done_by: "handled" }); });
    await waitFor(() => expect(shell.result.current.list.data).toEqual([]));
    await waitFor(() => expect(shell.result.current.summary.data).toEqual([]));
    const requests = listTodos.mock.calls.length;
    await act(async () => { reconnect(); });
    await waitFor(() => expect(listTodos.mock.calls.length).toBeGreaterThan(requests));
    await act(async () => { await handlers["todo:opened"]!({ todo: { id: 42 } }); });
    expect(shell.result.current.list.data).toEqual([]);
    shell.unmount();
    expect(handlers["todo:opened"]).toBeUndefined();
    expect(handlers["todo:closed"]).toBeUndefined();
    qc.clear();
  });

  it("removes a hidden review from the list and count from its event alone, also when the refresh fails", async () => {
    const qc = createQueryClient();
    qc.setQueryDefaults(["todos"], { retry: false });
    const review: Todo = { ...todo, id: "review-1", kind: "review", workspace_id: "ws-active", question_id: null, question_answer_type: null };
    let offline = false;
    setApiInstance({
      listTodos: async () => { if (offline) throw new Error("offline"); return [review]; },
      getTodoSummary: async () => { if (offline) throw new Error("offline"); return [{ workspace_id: "ws-active", count: 1 }]; },
    } as unknown as ApiClient);
    const handlers: Record<string, (payload: unknown) => void | Promise<void>> = {};
    const ws = {
      on: (event: string, handler: (payload: unknown) => void | Promise<void>) => {
        handlers[event] = handler;
        return () => { delete handlers[event]; };
      },
      onAny: () => () => {},
      onReconnect: () => () => {},
    } as unknown as WSClient;
    const stores = { authStore: { getState: () => ({ user: { id: "member-1" } }) } } as unknown as RealtimeSyncStores;
    function Wrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    }
    const shell = renderHook(() => {
      useRealtimeSync(ws, stores);
      return { list: useQuery(todoListOptions("ws-active")), summary: useQuery(todoSummaryOptions()) };
    }, { wrapper: Wrapper });
    await waitFor(() => expect(shell.result.current.list.data).toEqual([review]));
    await waitFor(() => expect(shell.result.current.summary.data?.[0]?.count).toBe(1));
    offline = true;
    await act(async () => { await handlers["todo:closed"]!({ member_id: "member-1", todo_id: review.id, kind: "review", done_by: null }); });
    await waitFor(() => expect([shell.result.current.list.isError, shell.result.current.summary.isError]).toEqual([true, true]));
    expect(shell.result.current.list.data).toEqual([]);
    expect(shell.result.current.summary.data).toEqual([{ workspace_id: "ws-active", count: 0 }]);
    shell.unmount();
    qc.clear();
  });
});
