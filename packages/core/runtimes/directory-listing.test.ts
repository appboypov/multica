// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import type { RuntimeDirectoryListingRequest } from "../types";
import {
  resolveRuntimeDirectoryListing,
  runtimeDirectoryListingKeys,
  runtimeDirectoryListingOptions,
} from "./directory-listing";

vi.mock("@multica/core/api", () => ({
  api: {
    initiateDirectoryListing: vi.fn(),
    getDirectoryListing: vi.fn(),
  },
}));

const initiateDirectoryListing = vi.mocked(api.initiateDirectoryListing);
const getDirectoryListing = vi.mocked(api.getDirectoryListing);
const entries = [
  { name: "reviewer", path: "/skills/reviewer", kind: "folder" as const, hidden: false },
  { name: ".notes.md", path: "/skills/.notes.md", kind: "file" as const, hidden: true },
];

function request(
  overrides: Partial<RuntimeDirectoryListingRequest> = {},
): RuntimeDirectoryListingRequest {
  return {
    id: "listing-1",
    runtime_id: "runtime-1",
    status: "pending",
    path: "/skills",
    entries: [],
    error: "",
    created_at: "2026-10-13T00:00:00Z",
    updated_at: "2026-10-13T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("resolveRuntimeDirectoryListing", () => {
  it("polls pending and running requests every 500 ms and returns the resolved path and entries", async () => {
    initiateDirectoryListing.mockResolvedValue(request());
    getDirectoryListing
      .mockResolvedValueOnce(request({ status: "running" }))
      .mockResolvedValueOnce(request({ status: "completed", entries }));

    const pending = resolveRuntimeDirectoryListing("runtime-1", "~/skills");
    await vi.advanceTimersByTimeAsync(499);
    expect(getDirectoryListing).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(getDirectoryListing).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(500);

    await expect(pending).resolves.toEqual({ path: "/skills", entries });
    expect(initiateDirectoryListing).toHaveBeenCalledWith("runtime-1", "~/skills");
    expect(getDirectoryListing).toHaveBeenCalledTimes(2);
    expect(getDirectoryListing).toHaveBeenLastCalledWith("runtime-1", "listing-1");
  });

  it("returns an immediately completed roots listing without polling", async () => {
    initiateDirectoryListing.mockResolvedValue(request({ status: "completed", path: "", entries }));

    await expect(resolveRuntimeDirectoryListing("runtime-1", "")).resolves.toEqual({ path: "", entries });
    expect(getDirectoryListing).not.toHaveBeenCalled();
  });

  it.each(["failed", "timeout"] as const)("throws the request error for %s", async (status) => {
    initiateDirectoryListing.mockResolvedValue(request({ status, error: "path does not exist: /missing" }));

    await expect(resolveRuntimeDirectoryListing("runtime-1", "/missing"))
      .rejects.toThrow("path does not exist: /missing");
    expect(getDirectoryListing).not.toHaveBeenCalled();
  });

  it("throws an error returned during polling", async () => {
    initiateDirectoryListing.mockResolvedValue(request());
    getDirectoryListing.mockResolvedValue(request({ status: "failed", error: "not a directory" }));

    const rejected = expect(resolveRuntimeDirectoryListing("runtime-1", "/file"))
      .rejects.toThrow("not a directory");
    await vi.advanceTimersByTimeAsync(500);
    await rejected;
  });

  it("uses a failure message when the request has no error", async () => {
    initiateDirectoryListing.mockResolvedValue(request({ status: "failed" }));

    await expect(resolveRuntimeDirectoryListing("runtime-1", "/skills"))
      .rejects.toThrow(/directory listing failed/);
  });

  it("times out at 30 seconds if the request is still pending", async () => {
    initiateDirectoryListing.mockResolvedValue(request());
    getDirectoryListing.mockResolvedValue(request());

    const rejected = expect(resolveRuntimeDirectoryListing("runtime-1", "/skills"))
      .rejects.toThrow(/timed out/);
    await vi.advanceTimersByTimeAsync(30_000);
    await rejected;
    expect(getDirectoryListing).toHaveBeenCalledTimes(60);
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("runtimeDirectoryListingOptions", () => {
  it("keys listings by runtime and path and makes each reopening fetch without retries", () => {
    const options = runtimeDirectoryListingOptions("runtime-1", "/skills");

    expect(runtimeDirectoryListingKeys.forPath("runtime-1", "/skills")).toEqual([
      "runtimes", "directory-listing", "runtime-1", "/skills",
    ]);
    expect(options.queryKey).toEqual(runtimeDirectoryListingKeys.forPath("runtime-1", "/skills"));
    expect(options.staleTime).toBe(0);
    expect(options.retry).toBe(false);
  });
});
