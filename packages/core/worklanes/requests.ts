import { queryOptions } from "@tanstack/react-query";
import { api, ApiError } from "../api";
import { WorklaneRequestReasonSchema } from "../api/schemas";
import type { WorklaneRequestReason } from "../types";

export const worklaneRequestKeys = {
  all: (wsId: string) => ["worklane-requests", wsId] as const,
  detail: (wsId: string, id: string) => [...worklaneRequestKeys.all(wsId), id] as const,
};

/** How often a running request is polled. */
export const WORKLANE_REQUEST_POLL_MS = 1000;

/**
 * A create or remove request on its machine, polled every second while it
 * runs. A finished request is never refetched: its outcome is final until a
 * retry starts it again.
 */
export function worklaneRequestOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: worklaneRequestKeys.detail(wsId, id),
    queryFn: () => api.getWorklaneRequest(id),
    enabled: id !== "",
    refetchInterval: (query) =>
      query.state.data?.status === "running" ? WORKLANE_REQUEST_POLL_MS : false,
    staleTime: Infinity,
  });
}

/**
 * The server's reason when it refuses a worklane call before anything runs
 * (`{error, code, detail}`), or null for any other error.
 */
export function worklaneRefusal(error: unknown): WorklaneRequestReason | null {
  if (!(error instanceof ApiError) || error.body === undefined) return null;
  const parsed = WorklaneRequestReasonSchema.safeParse(error.body);
  if (!parsed.success) return null;
  return parsed.data;
}
