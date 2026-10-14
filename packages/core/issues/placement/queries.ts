import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { api } from "../../api";

/** The issues of one worklane, every status, for its Timeline. */
export const worklaneIssueKeys = {
  all: (wsId: string) => ["worklane-issues", wsId] as const,
  list: (wsId: string, worklaneId: string) => [...worklaneIssueKeys.all(wsId), worklaneId] as const,
};

/** The done and total issues of every worklane and release of the workspace. */
export const releaseProgressKeys = {
  list: (wsId: string) => ["release-progress", wsId] as const,
};

export function worklaneIssuesOptions(wsId: string, worklaneId: string) {
  return queryOptions({
    queryKey: worklaneIssueKeys.list(wsId, worklaneId),
    queryFn: () => api.listWorklaneIssues(worklaneId),
    enabled: Boolean(worklaneId),
  });
}

export function releaseProgressOptions(wsId: string) {
  return queryOptions({
    queryKey: releaseProgressKeys.list(wsId),
    queryFn: () => api.listReleaseProgress(),
  });
}

/** Worklane issues and progress move with every issue, placement or release write. */
export function invalidatePlacementReads(qc: QueryClient, wsId: string) {
  qc.invalidateQueries({ queryKey: worklaneIssueKeys.all(wsId) });
  qc.invalidateQueries({ queryKey: releaseProgressKeys.list(wsId) });
}
