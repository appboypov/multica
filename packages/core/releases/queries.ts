import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { Release, ReleaseOwnerActivityEntry } from "../types";

export const releaseKeys = {
  all: (wsId: string) => ["releases", wsId] as const,
  /** One list for the whole workspace, by repository and position. */
  list: (wsId: string) => [...releaseKeys.all(wsId), "list"] as const,
  detailAll: (wsId: string) => [...releaseKeys.all(wsId), "detail"] as const,
  detail: (wsId: string, id: string) => [...releaseKeys.detailAll(wsId), id] as const,
  /** Every release's issues: an issue write can leave one release and join another. */
  issuesAll: (wsId: string) => [...releaseKeys.all(wsId), "issues"] as const,
  issues: (wsId: string, id: string) => [...releaseKeys.issuesAll(wsId), id] as const,
  commentsAll: (wsId: string) => [...releaseKeys.all(wsId), "comments"] as const,
  comments: (wsId: string, id: string) => [...releaseKeys.commentsAll(wsId), id] as const,
  activityAll: (wsId: string) => [...releaseKeys.all(wsId), "activity"] as const,
  activity: (wsId: string, id: string) => [...releaseKeys.activityAll(wsId), id] as const,
};

export function releaseListOptions(wsId: string) {
  return queryOptions({
    queryKey: releaseKeys.list(wsId),
    queryFn: () => api.listReleases(),
    select: (data) => data.releases,
  });
}

export function releaseDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: releaseKeys.detail(wsId, id),
    queryFn: () => api.getRelease(id),
  });
}

/** The release's issues in its order. */
export function releaseIssuesOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: releaseKeys.issues(wsId, id),
    queryFn: () => api.listReleaseIssues(id),
    select: (data) => data.issues,
  });
}

/** Comments and replies, oldest first. */
export function releaseCommentsOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: releaseKeys.comments(wsId, id),
    queryFn: () => api.listReleaseComments(id),
  });
}

/** The release's owner changes, oldest first. */
export function releaseActivityOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: releaseKeys.activity(wsId, id),
    queryFn: () => api.listReleaseActivity(id),
    select: (data): ReleaseOwnerActivityEntry[] => data.entries,
  });
}

const lineCache = new WeakMap<readonly Release[], Map<string, Release[]>>();

/**
 * The releases of one repository in line order. Memoised per list and
 * repository, so the reference stays stable while the cached list does.
 */
export function selectLine(releases: readonly Release[], repositoryId: string): Release[] {
  let byRepository = lineCache.get(releases);
  if (!byRepository) {
    byRepository = new Map();
    lineCache.set(releases, byRepository);
  }
  let line = byRepository.get(repositoryId);
  if (!line) {
    line = releases
      .filter((release) => release.repository_id === repositoryId)
      .sort((a, b) => a.position - b.position);
    byRepository.set(repositoryId, line);
  }
  return line;
}
