// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { InfiniteQueryObserver, QueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { ApiClient } from "../api/client";
import type { AgentSession, AgentTask } from "../types";
import {
  agentSessionsOptions,
  agentTasksKeys,
  agentTasksOptions,
} from "./queries";

vi.mock("../api", () => ({
  api: { listAgentTasksPage: vi.fn(), listAgentSessions: vi.fn() },
}));

const clients: QueryClient[] = [];
function client() {
  const result = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(result);
  return result;
}
function page(ids: string[], nextCursor: string | null = null) {
  const tasks = ids.map((id): AgentTask => ({
    id, agent_id: "agent-1", runtime_id: "runtime-1", issue_id: "",
    status: "completed", priority: 0, dispatched_at: null, started_at: null,
    completed_at: "2026-09-24T00:00:00Z", created_at: "2026-09-24T00:00:00Z",
    result: null, error: null,
  }));
  return { tasks, nextCursor };
}

afterEach(() => {
  clients.splice(0).forEach((queryClient) => queryClient.clear());
  vi.resetAllMocks();
  vi.unstubAllGlobals();
});

describe("agent history query lifecycle", () => {
  it("shares concurrent readers without eagerly fetching the next page", async () => {
    const queryClient = client();
    const fetchPage = vi.mocked(api.listAgentTasksPage);
    fetchPage.mockResolvedValue(page(["first"], "older"));
    const options = agentTasksOptions("workspace-1", "agent-1");
    const [first, second] = await Promise.all([
      queryClient.fetchInfiniteQuery(options),
      queryClient.fetchInfiniteQuery(options),
    ]);
    expect(fetchPage).toHaveBeenCalledTimes(1);
    expect(fetchPage).toHaveBeenCalledWith("agent-1", {
      limit: 200, before: undefined, signal: expect.any(AbortSignal),
    });
    expect(first).toEqual(second);
    expect(first.pages).toHaveLength(1);
    expect(first.pages[0]?.nextCursor).toBe("older");
  });

  it("isolates cached pages by workspace and agent", async () => {
    const queryClient = client();
    const fetchPage = vi.mocked(api.listAgentTasksPage);
    fetchPage.mockResolvedValueOnce(page(["workspace-1-agent-1"]))
      .mockResolvedValueOnce(page(["workspace-2-agent-1"]))
      .mockResolvedValueOnce(page(["workspace-1-agent-2"]));
    const first = await queryClient.fetchInfiniteQuery(agentTasksOptions("workspace-1", "agent-1"));
    const otherWorkspace = await queryClient.fetchInfiniteQuery(agentTasksOptions("workspace-2", "agent-1"));
    const otherAgent = await queryClient.fetchInfiniteQuery(agentTasksOptions("workspace-1", "agent-2"));
    const cached = await queryClient.fetchInfiniteQuery(agentTasksOptions("workspace-1", "agent-1"));
    expect(fetchPage).toHaveBeenCalledTimes(3);
    expect(cached).toEqual(first);
    expect(otherWorkspace.pages[0]?.tasks[0]?.id).toBe("workspace-2-agent-1");
    expect(otherAgent.pages[0]?.tasks[0]?.id).toBe("workspace-1-agent-2");
  });

  it("rebuilds subsequent cursors after a task event invalidates loaded pages", async () => {
    const queryClient = client();
    const fetchPage = vi.mocked(api.listAgentTasksPage);
    fetchPage.mockResolvedValueOnce(page(["first"], "old-boundary"))
      .mockResolvedValueOnce(page(["second"]))
      .mockResolvedValueOnce(page(["new"], "new-boundary"))
      .mockResolvedValueOnce(page(["first", "second"]));
    const options = agentTasksOptions("workspace-1", "agent-1");
    await queryClient.fetchInfiniteQuery(options);
    const observer = new InfiniteQueryObserver(queryClient, options);
    const unsubscribe = observer.subscribe(() => {});
    try {
      await observer.fetchNextPage();
      expect(fetchPage.mock.calls[1]?.[1]?.before).toBe("old-boundary");
      // This is the same prefix used by task lifecycle WebSocket events.
      await queryClient.invalidateQueries({ queryKey: agentTasksKeys.all("workspace-1") });
      expect(fetchPage).toHaveBeenCalledTimes(4);
      expect(fetchPage.mock.calls[2]?.[1]?.before).toBeUndefined();
      expect(fetchPage.mock.calls[3]?.[1]?.before).toBe("new-boundary");
      expect(observer.getCurrentResult().data?.pages.flatMap((p) => p.tasks.map((t) => t.id)))
        .toEqual(["new", "first", "second"]);
      expect(observer.getCurrentResult().hasNextPage).toBe(false);
      await observer.fetchNextPage();
      expect(fetchPage).toHaveBeenCalledTimes(4);
    } finally {
      unsubscribe();
    }
  });

  it("aborts a pending history request when its last reader leaves", async () => {
    const queryClient = client();
    let requestSignal: AbortSignal | undefined;
    vi.mocked(api.listAgentTasksPage).mockImplementation((_id, options) => {
      requestSignal = options?.signal;
      return new Promise((_resolve, reject) => {
        requestSignal?.addEventListener("abort", () => reject(new Error("aborted")), { once: true });
      });
    });
    const options = agentTasksOptions("workspace-1", "agent-1");
    const observer = new InfiniteQueryObserver(queryClient, options);
    const unsubscribe = observer.subscribe(() => {});
    await vi.waitFor(() => expect(requestSignal).toBeDefined());
    expect(requestSignal?.aborted).toBe(false);
    unsubscribe();
    expect(requestSignal?.aborted).toBe(true);
    expect(queryClient.getQueryState(options.queryKey)?.fetchStatus).toBe("idle");
    expect(queryClient.getQueryData(options.queryKey)).toBeUndefined();
  });
});

function session(id: string): AgentSession {
  return {
    id, kind: "terminal", pid: 42, herdr_pane_id: "w12:p0", runtime: null,
    task_id: null, issue: null, work_dir: "/Users/brian/Repos/multica",
    repository: "appboypov/multica", branch: "work-1089", status: "working",
    started_at: "2026-10-15T18:30:00Z",
  };
}

describe("agent sessions query", () => {
  it("isolates cached sessions by workspace and agent", async () => {
    const queryClient = client();
    const listSessions = vi.mocked(api.listAgentSessions);
    listSessions.mockResolvedValueOnce({ sessions: [session("workspace-1-agent-1")] })
      .mockResolvedValueOnce({ sessions: [session("workspace-2-agent-1")] })
      .mockResolvedValueOnce({ sessions: [session("workspace-1-agent-2")] });
    const first = await queryClient.fetchQuery(agentSessionsOptions("workspace-1", "agent-1"));
    const otherWorkspace = await queryClient.fetchQuery(agentSessionsOptions("workspace-2", "agent-1"));
    const otherAgent = await queryClient.fetchQuery(agentSessionsOptions("workspace-1", "agent-2"));
    const cached = await queryClient.fetchQuery(agentSessionsOptions("workspace-1", "agent-1"));
    expect(listSessions).toHaveBeenCalledTimes(3);
    expect(listSessions).toHaveBeenCalledWith("agent-1", { signal: expect.any(AbortSignal) });
    expect(cached).toEqual(first);
    expect(otherWorkspace.sessions[0]?.id).toBe("workspace-2-agent-1");
    expect(otherAgent.sessions[0]?.id).toBe("workspace-1-agent-2");
  });

  it("reads a malformed response as a usable list", async () => {
    const queryClient = client();
    const real = new ApiClient("https://api.example.test");
    vi.mocked(api.listAgentSessions).mockImplementation((agentId, options) =>
      real.listAgentSessions(agentId, options));
    vi.stubGlobal("fetch", vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ other: [] })))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        sessions: [{ ...session("from-newer-server"), status: "paused" }],
      }))));
    const missing = await queryClient.fetchQuery(agentSessionsOptions("workspace-1", "agent-1"));
    const unknownStatus = await queryClient.fetchQuery(agentSessionsOptions("workspace-1", "agent-2"));
    expect(missing.sessions).toEqual([]);
    expect(unknownStatus.sessions).toEqual([
      { ...session("from-newer-server"), status: "unknown" },
    ]);
  });
});
