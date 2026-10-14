import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { CreateRepositoryCheckoutRequest, RepositoryCheckoutRequest } from "../types";

export const repositoryCheckoutKeys = {
  all: () => ["repositories", "checkout"] as const,
  check: (runtimeId: string, fullName: string, defaultBranch: string, path: string) =>
    [...repositoryCheckoutKeys.all(), runtimeId, fullName, defaultBranch, path] as const,
};

const POLL_INTERVAL_MS = 500;
/** A check reads only what is on the machine; a clone downloads the repository. */
const CHECK_TIMEOUT_MS = 30_000;
const CLONE_TIMEOUT_MS = 10 * 60_000;

/**
 * Runs a checkout check, or a clone then the check, on a runtime and polls it
 * as `resolveRuntimeDirectoryListing` does. A completed request is returned
 * whether it passed or not; its `failures` say why. A request the daemon could
 * not finish, or one past its timeout, throws with the server's error.
 */
export async function resolveRepositoryCheckout(
  runtimeId: string,
  data: CreateRepositoryCheckoutRequest,
): Promise<RepositoryCheckoutRequest> {
  const initial = await api.initiateRepositoryCheckout(runtimeId, data);
  const timeoutMs = data.clone ? CLONE_TIMEOUT_MS : CHECK_TIMEOUT_MS;
  const start = Date.now();
  let current = initial;

  while (current.status === "pending" || current.status === "running") {
    if (Date.now() - start >= timeoutMs) {
      throw new Error("repository checkout timed out");
    }
    const { promise, resolve } = Promise.withResolvers<void>();
    setTimeout(resolve, POLL_INTERVAL_MS);
    await promise;
    current = await api.getRepositoryCheckout(runtimeId, initial.id);
  }

  if (current.status === "failed" || current.status === "timeout") {
    throw new Error(current.error || "repository checkout failed");
  }
  return current;
}

/** The check of one path for one repository; never cached across opens. */
export function repositoryCheckoutOptions(
  runtimeId: string,
  fullName: string,
  defaultBranch: string,
  path: string,
) {
  return queryOptions({
    queryKey: repositoryCheckoutKeys.check(runtimeId, fullName, defaultBranch, path),
    queryFn: () =>
      resolveRepositoryCheckout(runtimeId, {
        full_name: fullName,
        default_branch: defaultBranch,
        path,
        clone: false,
      }),
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  });
}
