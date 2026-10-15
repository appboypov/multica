/**
 * @vitest-environment jsdom
 */
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { WSClient } from "../api/ws-client";
import { issueKeys, type IssueSortParam } from "../issues/queries";
import type { Issue, ListIssuesCache, RunInputReactedPayload, TimelineEntry } from "../types";
import { questionKeys } from "../questions/queries";
import { useRealtimeSync, type RealtimeSyncStores } from "./use-realtime-sync";

vi.mock("../platform/workspace-storage", () => ({
  getCurrentWsId: () => "ws-1",
  getCurrentSlug: () => "test-ws",
  // Draft stores are now loaded transitively (storage-cleanup → register-all-drafts)
  // so their persist wiring must resolve against this mock.
  createWorkspaceAwareStorage: (adapter: unknown) => adapter,
  registerForWorkspaceRehydration: () => {},
}));

vi.mock("../paths", () => ({
  useHasOnboarded: () => true,
  resolvePostAuthDestination: () => "/",
}));

// Records every ws.on handler by event name so a test can fire one directly.
function createRecordingWs(): {
  ws: WSClient;
  handlers: Record<string, (p: unknown) => void>;
} {
  const handlers: Record<string, (p: unknown) => void> = {};
  const ws = {
    on: vi.fn((event: string, handler: (p: unknown) => void) => {
      handlers[event] = handler;
      return () => { delete handlers[event]; };
    }),
    onAny: vi.fn(() => () => {}),
    onReconnect: vi.fn(() => () => {}),
  } as unknown as WSClient;
  return { ws, handlers };
}

function createStores(): RealtimeSyncStores {
  return {
    authStore: Object.assign(() => ({}), {
      getState: () => ({ user: { id: "u1" } }),
      subscribe: () => () => {},
      setState: () => {},
      destroy: () => {},
    }),
  } as unknown as RealtimeSyncStores;
}

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

const updatedSort: IssueSortParam = {
  sort_by: "updated_at",
  sort_direction: "desc",
};
const positionSort: IssueSortParam = { sort_by: "position" };
const updatedBoardKey = issueKeys.listSorted("ws-1", updatedSort);
const positionBoardKey = issueKeys.listSorted("ws-1", positionSort);
const lastActivityBoardKey = issueKeys.listSorted("ws-1", {
  sort_by: "last_activity",
  sort_direction: "desc",
});

function bucketed(): ListIssuesCache {
  return { byStatus: { unstarted: { issues: [], total: 1 } } };
}

describe("useRealtimeSync — comment activity cache coherence", () => {
  let qc: QueryClient;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  });

  afterEach(() => {
    qc.clear();
    vi.clearAllMocks();
  });

  it("invalidates the updated_at-sorted board but leaves the position board", () => {
    qc.setQueryData<ListIssuesCache>(updatedBoardKey, bucketed());
    qc.setQueryData<ListIssuesCache>(positionBoardKey, bucketed());
    qc.setQueryData<ListIssuesCache>(lastActivityBoardKey, bucketed());
    qc.setQueryData(issueKeys.timeline("issue-1"), []);

    const { ws, handlers } = createRecordingWs();
    renderHook(() => useRealtimeSync(ws, createStores()), {
      wrapper: createWrapper(qc),
    });

    handlers["comment:created"]?.({
      comment: { id: "c1", issue_id: "issue-1" },
    });

    expect(qc.getQueryState(updatedBoardKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(lastActivityBoardKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(positionBoardKey)?.isInvalidated).toBe(false);
    // The per-issue timeline is still invalidated as before.
    expect(
      qc.getQueryState(issueKeys.timeline("issue-1"))?.isInvalidated,
    ).toBe(true);
  });

  it("uses update/delete owner revisions and re-sorts only last_activity", () => {
    const detailKey = issueKeys.detail("ws-1", "issue-1");
    qc.setQueryData<Issue>(detailKey, {
      id: "issue-1",
      workspace_id: "ws-1",
      revision: 1,
    } as Issue);
    qc.setQueryData<ListIssuesCache>(updatedBoardKey, bucketed());
    qc.setQueryData<ListIssuesCache>(positionBoardKey, bucketed());
    qc.setQueryData<ListIssuesCache>(lastActivityBoardKey, bucketed());
    qc.setQueryData(issueKeys.timeline("issue-1"), []);

    const { ws, handlers } = createRecordingWs();
    renderHook(() => useRealtimeSync(ws, createStores()), {
      wrapper: createWrapper(qc),
    });

    handlers["comment:updated"]?.({
      comment: { id: "c1", issue_id: "issue-1" },
      issue_revision: 2,
    });

    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(lastActivityBoardKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(updatedBoardKey)?.isInvalidated).toBe(false);
    expect(qc.getQueryState(positionBoardKey)?.isInvalidated).toBe(false);

    qc.setQueryData<Issue>(detailKey, {
      id: "issue-1",
      workspace_id: "ws-1",
      revision: 2,
    } as Issue);
    qc.setQueryData<ListIssuesCache>(lastActivityBoardKey, bucketed());

    handlers["comment:deleted"]?.({
      comment_id: "c1",
      issue_id: "issue-1",
      issue_revision: 3,
    });

    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(lastActivityBoardKey)?.isInvalidated).toBe(true);
  });

  it("ignores a comment event with no issue_id", () => {
    qc.setQueryData<ListIssuesCache>(updatedBoardKey, bucketed());

    const { handlers } = (() => {
      const rec = createRecordingWs();
      renderHook(() => useRealtimeSync(rec.ws, createStores()), {
        wrapper: createWrapper(qc),
      });
      return rec;
    })();

    handlers["comment:created"]?.({ comment: { id: "c1" } });

    expect(qc.getQueryState(updatedBoardKey)?.isInvalidated).toBe(false);
  });

  it.each(["comment", "comment_edit", "answer", "issue_change"] as const)(
    "refreshes the item's timeline and workspace question projections for %s marks",
    (item_kind) => {
      qc.setQueryData(issueKeys.timeline("issue-1"), []);
      qc.setQueryData(issueKeys.timeline("other-issue"), []);
      const keys = [
        questionKeys.byIssue("ws-1", "issue-1"),
        questionKeys.byIssue("ws-1", "linked-issue"),
        questionKeys.detail("ws-1", "question-1"),
        questionKeys.rowsAll("ws-1"),
        questionKeys.openCount("ws-1"),
      ];
      keys.forEach((key) => qc.setQueryData(key, []));
      qc.setQueryData(questionKeys.byIssue("ws-2", "issue-1"), []);
      const { ws, handlers } = createRecordingWs();
      const { unmount } = renderHook(() => useRealtimeSync(ws, createStores()), { wrapper: createWrapper(qc) });

      const payload: RunInputReactedPayload = {
        issue_id: "issue-1", item_kind, item_id: "item-1", task_id: "run-1",
        agent_id: "thomas", seen_at: "2026-10-15T10:01:00Z", handled_at: null,
      };
      handlers["run_input:reacted"]!(payload);

      expect(qc.getQueryState(issueKeys.timeline("issue-1"))?.isInvalidated).toBe(true);
      expect(qc.getQueryState(issueKeys.timeline("other-issue"))?.isInvalidated).toBe(false);
      for (const key of keys) expect(qc.getQueryState(key)?.isInvalidated).toBe(true);
      expect(qc.getQueryState(questionKeys.byIssue("ws-2", "issue-1"))?.isInvalidated).toBe(false);
      unmount();
      expect(handlers["run_input:reacted"]).toBeUndefined();
    },
  );

  it("refetches an actively watched timeline so marks appear without a reload", async () => {
    const entry: TimelineEntry = {
      type: "activity", id: "activity-1", actor_type: "member", actor_id: "brian",
      action: "priority_changed", created_at: "2026-10-15T10:00:00Z",
    };
    const marked = {
      ...entry,
      run_marks: [{ task_id: "run-1", agent_id: "thomas", handled_at: "2026-10-15T10:02:00Z" }],
    };
    const fetchTimeline = vi.fn().mockResolvedValue([marked]);
    const { ws, handlers } = createRecordingWs();
    const stores = createStores();
    const { result } = renderHook(() => {
      useRealtimeSync(ws, stores);
      return useQuery({
        queryKey: issueKeys.timeline("issue-1"), queryFn: fetchTimeline,
        initialData: [entry], staleTime: Infinity,
      });
    }, { wrapper: createWrapper(qc) });
    expect(fetchTimeline).not.toHaveBeenCalled();

    act(() => {
      handlers["run_input:reacted"]!({
        issue_id: "issue-1", item_kind: "issue_change", item_id: entry.id,
        task_id: "run-1", agent_id: "thomas", seen_at: null, handled_at: "2026-10-15T10:02:00Z",
      });
    });

    await waitFor(() => expect(result.current.data).toEqual([marked]));
    expect(fetchTimeline).toHaveBeenCalledTimes(1);
  });

  it("ignores a run input mark with no issue identity", () => {
    qc.setQueryData(issueKeys.timeline("issue-1"), []);
    qc.setQueryData(questionKeys.byIssue("ws-1", "issue-1"), []);
    const { ws, handlers } = createRecordingWs();
    renderHook(() => useRealtimeSync(ws, createStores()), { wrapper: createWrapper(qc) });

    handlers["run_input:reacted"]!({ item_id: "item-1" });

    expect(qc.getQueryState(issueKeys.timeline("issue-1"))?.isInvalidated).toBe(false);
    expect(qc.getQueryState(questionKeys.byIssue("ws-1", "issue-1"))?.isInvalidated).toBe(false);
  });
});
