import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "../api";

/**
 * Every worklane query of a workspace sits under `all`, so one invalidation
 * refreshes the list, each detail and each worklane's activity.
 * `activityAll` is the prefix of every worklane's activity, for writes that
 * change repository-wide entries (ADR-0008).
 */
export const worklaneKeys = {
  all: (wsId: string) => ["worklanes", wsId] as const,
  list: (wsId: string) => [...worklaneKeys.all(wsId), "list"] as const,
  detail: (wsId: string, id: string) => [...worklaneKeys.all(wsId), "detail", id] as const,
  activityAll: (wsId: string) => [...worklaneKeys.all(wsId), "activity"] as const,
  activity: (wsId: string, id: string) => [...worklaneKeys.activityAll(wsId), id] as const,
};

/** Every worklane of the workspace, each repository's `main` first. */
export function worklaneListOptions(wsId: string) {
  return queryOptions({
    queryKey: worklaneKeys.list(wsId),
    queryFn: () => api.listWorklanes(),
    select: (data) => data.worklanes,
  });
}

export function worklaneDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: worklaneKeys.detail(wsId, id),
    queryFn: () => api.getWorklane(id),
  });
}

/** The newest 200 entries of the worklane and its repository, oldest first. */
export function worklaneActivityOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: worklaneKeys.activity(wsId, id),
    queryFn: () => api.listWorklaneActivity(id),
    select: (data) => data.entries,
  });
}

export function useWorklanes(wsId: string) {
  return useQuery(worklaneListOptions(wsId));
}

export function useWorklane(wsId: string, id: string) {
  return useQuery(worklaneDetailOptions(wsId, id));
}

export function useWorklaneActivity(wsId: string, id: string) {
  return useQuery(worklaneActivityOptions(wsId, id));
}
