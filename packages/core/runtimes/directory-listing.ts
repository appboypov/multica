import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { RuntimeDirectoryEntry } from "../types";

export const runtimeDirectoryListingKeys = {
  all: () => ["runtimes", "directory-listing"] as const,
  forPath: (runtimeId: string, path: string) =>
    [...runtimeDirectoryListingKeys.all(), runtimeId, path] as const,
};

const POLL_INTERVAL_MS = 500;
const POLL_TIMEOUT_MS = 30_000;

export async function resolveRuntimeDirectoryListing(
  runtimeId: string,
  path: string,
): Promise<{ path: string; entries: RuntimeDirectoryEntry[] }> {
  const initial = await api.initiateDirectoryListing(runtimeId, path);
  const start = Date.now();
  let current = initial;

  while (current.status === "pending" || current.status === "running") {
    if (Date.now() - start >= POLL_TIMEOUT_MS) {
      throw new Error("runtime directory listing timed out");
    }
    const { promise, resolve } = Promise.withResolvers<void>();
    setTimeout(resolve, POLL_INTERVAL_MS);
    await promise;
    current = await api.getDirectoryListing(runtimeId, initial.id);
  }

  if (current.status === "failed" || current.status === "timeout") {
    throw new Error(current.error || "runtime directory listing failed");
  }

  return { path: current.path, entries: current.entries };
}

export function runtimeDirectoryListingOptions(runtimeId: string, path: string) {
  return queryOptions({
    queryKey: runtimeDirectoryListingKeys.forPath(runtimeId, path),
    queryFn: () => resolveRuntimeDirectoryListing(runtimeId, path),
    staleTime: 0,
    retry: false,
  });
}
