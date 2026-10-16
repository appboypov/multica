/**
 * @vitest-environment jsdom
 */
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError, setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { EMPTY_RELEASE } from "../api/schemas";
import type { ListReleasesResponse, Release, ReleaseComment } from "../types";
import { useCreateReleaseComment, useSetReleaseOwner } from "./mutations";
import { releaseDetailOptions, releaseKeys } from "./queries";

vi.mock("../hooks", () => ({ useWorkspaceId: () => "ws-a" }));

const release = (overrides: Partial<Release>): Release => ({
  ...EMPTY_RELEASE,
  repository_id: "repo-1",
  ...overrides,
});
const planned = release({ id: "r-2", position: 1, version: "1.1.0", title: "Account and profile", description: "Fans sign in" });
const shipped = release({ id: "r-1", position: 0, version: "1.0.0", title: "First", shipped_at: "2026-01-01T00:00:00Z" });
const otherRepository = release({ id: "r-9", repository_id: "repo-2", version: "3.0.0", title: "Other" });

function client() {
  return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
}

function wrapper(qc: QueryClient) {
  return ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: qc }, children);
}

beforeEach(() => vi.restoreAllMocks());

describe("release page reads", () => {
  it("keeps the same release id apart per workspace", async () => {
    const getRelease = vi.fn().mockResolvedValue(planned);
    setApiInstance({ getRelease } as unknown as ApiClient);
    const qc = client();
    await qc.fetchQuery(releaseDetailOptions("ws-a", "r-2"));
    expect(qc.getQueryData(releaseKeys.detail("ws-a", "r-2"))).toEqual(planned);
    expect(qc.getQueryData(releaseKeys.detail("ws-b", "r-2"))).toBeUndefined();
    expect(releaseKeys.detail("ws-b", "r-2")).not.toEqual(releaseKeys.detail("ws-a", "r-2"));
    for (const key of [releaseKeys.issues, releaseKeys.comments, releaseKeys.activity]) {
      expect(key("ws-a", "r-2")).not.toEqual(key("ws-b", "r-2"));
    }
  });
});

describe("useSetReleaseOwner", () => {
  it("settles the whole line in the list and on the open release page", async () => {
    const owned = { ...planned, owner_type: "agent" as const, owner_id: "agent-1", owner_name: "Pieter" };
    const setReleaseOwner = vi.fn().mockResolvedValue({ releases: [shipped, owned] });
    setApiInstance({ setReleaseOwner } as unknown as ApiClient);
    const qc = client();
    qc.setQueryData<ListReleasesResponse>(releaseKeys.list("ws-a"), { releases: [shipped, planned, otherRepository] });
    qc.setQueryData(releaseKeys.detail("ws-a", "r-2"), planned);

    const { result } = renderHook(() => useSetReleaseOwner(), { wrapper: wrapper(qc) });
    await result.current.mutateAsync({
      id: "r-2",
      repositoryId: "repo-1",
      owner: { owner_type: "agent", owner_id: "agent-1" },
    });

    expect(setReleaseOwner).toHaveBeenCalledWith("r-2", { owner_type: "agent", owner_id: "agent-1" });
    expect(qc.getQueryData<ListReleasesResponse>(releaseKeys.list("ws-a"))?.releases).toEqual([
      otherRepository,
      shipped,
      owned,
    ]);
    expect(qc.getQueryData(releaseKeys.detail("ws-a", "r-2"))).toEqual(owned);
    // A release page that is not open is not created by the answer.
    expect(qc.getQueryData(releaseKeys.detail("ws-a", "r-1"))).toBeUndefined();
  });

  it("leaves the cached owner as it was when the server refuses", async () => {
    const refusal = new ApiError("v1.0.0 First is shipped", 409, "Conflict", { code: "release_shipped" });
    setApiInstance({ setReleaseOwner: vi.fn().mockRejectedValue(refusal) } as unknown as ApiClient);
    const qc = client();
    qc.setQueryData(releaseKeys.detail("ws-a", "r-1"), shipped);
    const { result } = renderHook(() => useSetReleaseOwner(), { wrapper: wrapper(qc) });
    await expect(
      result.current.mutateAsync({ id: "r-1", repositoryId: "repo-1", owner: { owner_type: null, owner_id: null } }),
    ).rejects.toBe(refusal);
    expect(qc.getQueryData(releaseKeys.detail("ws-a", "r-1"))).toEqual(shipped);
  });
});

describe("useCreateReleaseComment", () => {
  const comment: ReleaseComment = {
    id: "c-1",
    release_id: "r-2",
    author_type: "member",
    author_id: "user-1",
    content: "Start WORK-502 once WORK-501's pull request is open.",
    parent_id: null,
    attachments: [],
    created_at: "2026-01-02T00:00:00Z",
    updated_at: "2026-01-02T00:00:00Z",
    source_task_id: null,
  };

  it("shows a sent comment once, even when the realtime refresh already brought it", async () => {
    const createReleaseComment = vi.fn().mockResolvedValue(comment);
    const listReleaseComments = vi.fn().mockResolvedValue([comment]);
    setApiInstance({ createReleaseComment, listReleaseComments } as unknown as ApiClient);
    const qc = client();
    qc.setQueryData<ReleaseComment[]>(releaseKeys.comments("ws-a", "r-2"), [comment]);
    const { result } = renderHook(() => useCreateReleaseComment("r-2"), { wrapper: wrapper(qc) });
    await result.current.mutateAsync({ content: comment.content });
    expect(createReleaseComment).toHaveBeenCalledWith("r-2", { content: comment.content });
    expect(qc.getQueryData(releaseKeys.comments("ws-a", "r-2"))).toEqual([comment]);
  });

  it("rejects a failed send without adding anything, so the caller keeps the text", async () => {
    const failure = new Error("network down");
    setApiInstance({
      createReleaseComment: vi.fn().mockRejectedValue(failure),
      listReleaseComments: vi.fn().mockResolvedValue([]),
    } as unknown as ApiClient);
    const qc = client();
    qc.setQueryData<ReleaseComment[]>(releaseKeys.comments("ws-a", "r-2"), []);
    const { result } = renderHook(() => useCreateReleaseComment("r-2"), { wrapper: wrapper(qc) });
    await expect(result.current.mutateAsync({ content: "Hello" })).rejects.toBe(failure);
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(qc.getQueryData(releaseKeys.comments("ws-a", "r-2"))).toEqual([]);
  });
});
