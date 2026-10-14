import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { Release } from "../types";

export const releaseKeys = {
  all: (wsId: string) => ["releases", wsId] as const,
  /** One list for the whole workspace, by repository and position. */
  list: (wsId: string) => [...releaseKeys.all(wsId), "list"] as const,
};

export function releaseListOptions(wsId: string) {
  return queryOptions({
    queryKey: releaseKeys.list(wsId),
    queryFn: () => api.listReleases(),
    select: (data) => data.releases,
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
