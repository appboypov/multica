import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { onTodoClosed } from "@multica/core/todos/ws-updaters";
import type { TodoWorkspaceCount } from "@multica/core/types";
import { useDesktopTodoBadge } from "./use-desktop-todo-badge";

type DesktopWindow = { desktopAPI?: { setUnreadBadge: (count: number) => void } };

function setup(summaries: TodoWorkspaceCount[][]) {
  const setUnreadBadge = vi.fn();
  (window as unknown as DesktopWindow).desktopAPI = { setUnreadBadge };
  const getTodoSummary = vi.fn(async () => summaries.shift() ?? []);
  const getInboxUnreadSummary = vi.fn(async () => [{ workspace_id: "ws-1", count: 9 }]);
  setApiInstance({ getTodoSummary, getInboxUnreadSummary } as unknown as ApiClient);
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return { qc, setUnreadBadge, getTodoSummary, getInboxUnreadSummary, wrapper };
}

afterEach(() => {
  cleanup();
  delete (window as unknown as DesktopWindow).desktopAPI;
});

describe("useDesktopTodoBadge", () => {
  it("shows the active workspace's open to-dos, not unread Updates or other workspaces", async () => {
    const { setUnreadBadge, getInboxUnreadSummary, wrapper } = setup([
      [{ workspace_id: "ws-1", count: 3 }, { workspace_id: "ws-2", count: 5 }],
    ]);

    renderHook(() => useDesktopTodoBadge("ws-1"), { wrapper });

    await waitFor(() => expect(setUnreadBadge).toHaveBeenLastCalledWith(3));
    expect(getInboxUnreadSummary).not.toHaveBeenCalled();
  });

  it("clears the badge when the workspace has no open to-dos", async () => {
    const { setUnreadBadge, wrapper } = setup([[{ workspace_id: "ws-2", count: 5 }]]);

    renderHook(() => useDesktopTodoBadge("ws-1"), { wrapper });

    await waitFor(() => expect(setUnreadBadge).toHaveBeenCalledWith(0));
    expect(setUnreadBadge).not.toHaveBeenCalledWith(5);
  });

  it("clears without a workspace and does not read the summary", () => {
    const { setUnreadBadge, getTodoSummary, wrapper } = setup([]);

    renderHook(() => useDesktopTodoBadge(null), { wrapper });

    expect(setUnreadBadge).toHaveBeenLastCalledWith(0);
    expect(getTodoSummary).not.toHaveBeenCalled();
  });

  it("follows a realtime close down to the server's new count", async () => {
    const { qc, setUnreadBadge, wrapper } = setup([
      [{ workspace_id: "ws-1", count: 2 }],
      [{ workspace_id: "ws-1", count: 1 }],
    ]);
    renderHook(() => useDesktopTodoBadge("ws-1"), { wrapper });
    await waitFor(() => expect(setUnreadBadge).toHaveBeenLastCalledWith(2));

    await act(async () => onTodoClosed(qc, "todo-1"));

    await waitFor(() => expect(setUnreadBadge).toHaveBeenLastCalledWith(1));
  });
});
