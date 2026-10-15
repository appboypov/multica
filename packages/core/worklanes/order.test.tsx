// @vitest-environment jsdom
import { type ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import type { Issue } from "../types";
import { worklaneIssueKeys } from "../issues/placement/queries";
import { moveLaneIssue, useMoveWorklaneIssue } from "./order";

vi.mock("../hooks", () => ({ useWorkspaceId: () => "ws" }));
vi.mock("../api", () => ({ api: { moveWorklaneIssue: vi.fn() } }));
afterEach(() => vi.restoreAllMocks());

function issue(id: string, release: string | null = null): Issue {
  return {
    id, workspace_id: "ws", number: 1, identifier: id, title: id, description: null,
    status: "todo", priority: "none", assignee_type: null, assignee_id: null,
    creator_type: "member", creator_id: "user", parent_issue_id: null,
    project_id: null, worklane_id: "lane", release_id: release, type: null,
    position: 0, stage: null, start_date: null, due_date: null,
    metadata: {}, properties: {}, created_at: "2026-10-15T00:00:00Z", updated_at: "2026-10-15T00:00:00Z",
  };
}
const ids = (issues: readonly Issue[]) => issues.map((issue) => issue.id);

describe("lane order", () => {
  it("moves before and after without changing board positions or release groups", () => {
    const issues = [issue("a", "release"), issue("b", "release"), issue("c")];
    expect(ids(moveLaneIssue(issues, { issueId: "b", before: "a" }))).toEqual(["b", "a", "c"]);
    expect(ids(moveLaneIssue(issues, { issueId: "a", after: "b" }))).toEqual(["b", "a", "c"]);
    expect(moveLaneIssue(issues, { issueId: "c", before: "a" })).toEqual(issues);
    expect(moveLaneIssue(issues, { issueId: "missing", before: "a" })).toEqual(issues);
    expect(issues.every((issue) => issue.position === 0)).toBe(true);
  });

  it("patches immediately and rolls the lane back when the server refuses", async () => {
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false } } });
    const queryKey = worklaneIssueKeys.list("ws", "lane");
    qc.setQueryData(queryKey, [issue("a"), issue("b")]);
    const { promise, reject } = Promise.withResolvers<Issue[]>();
    vi.spyOn(api, "moveWorklaneIssue").mockReturnValue(promise);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result } = renderHook(() => useMoveWorklaneIssue("lane"), { wrapper });
    act(() => result.current.mutate({ issueId: "b", before: "a" }));
    await waitFor(() => expect(ids(qc.getQueryData<Issue[]>(queryKey) ?? [])).toEqual(["b", "a"]));
    act(() => reject(new Error("refused")));
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(ids(qc.getQueryData<Issue[]>(queryKey) ?? [])).toEqual(["a", "b"]);
    qc.clear();
  });

  it("uses the canonical returned lane order after success", async () => {
    const qc = new QueryClient();
    const queryKey = worklaneIssueKeys.list("ws", "lane");
    qc.setQueryData(queryKey, [issue("a"), issue("b")]);
    vi.spyOn(api, "moveWorklaneIssue").mockResolvedValue([issue("b"), issue("a")]);
    const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    const { result } = renderHook(() => useMoveWorklaneIssue("lane"), { wrapper });
    await act(() => result.current.mutateAsync({ issueId: "b", before: "a" }));
    expect(ids(qc.getQueryData<Issue[]>(queryKey) ?? [])).toEqual(["b", "a"]);
    expect(api.moveWorklaneIssue).toHaveBeenCalledWith("lane", "b", { issueId: "b", before: "a" });
    qc.clear();
  });
});
