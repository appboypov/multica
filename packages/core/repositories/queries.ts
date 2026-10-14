import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { Repository } from "../types";

export const repositoryKeys = {
  all: (wsId: string) => ["repositories", wsId] as const,
  list: (wsId: string) => [...repositoryKeys.all(wsId), "list"] as const,
  available: (wsId: string) => [...repositoryKeys.all(wsId), "available"] as const,
};

/** Every repository of the workspace, as `Repository[]`. */
export function repositoryListOptions(wsId: string) {
  return queryOptions({
    queryKey: repositoryKeys.list(wsId),
    queryFn: () => api.listRepositories(),
    select: (data) => data.repositories,
  });
}

/**
 * The repositories the workspace's GitHub App installations can see, and the
 * App's installation page. Refetched when the window regains focus, so a
 * repository given access on GitHub shows when Brian comes back.
 */
export function availableRepositoriesOptions(wsId: string) {
  return queryOptions({
    queryKey: repositoryKeys.available(wsId),
    queryFn: () => api.listAvailableRepositories(),
    staleTime: 0,
    refetchOnWindowFocus: "always",
  });
}

/** A project's repositories, by `full_name` as its page shows them. */
export function repositoriesOfProject(
  repositories: readonly Repository[],
  projectId: string,
): Repository[] {
  return repositories
    .filter((repository) => repository.project_id === projectId)
    .sort(compareRepositories);
}

export function compareRepositories(a: Repository, b: Repository): number {
  return a.full_name.toLowerCase().localeCompare(b.full_name.toLowerCase());
}

/** The number of repositories of each project, by project id. */
export function countRepositoriesByProject(
  repositories: readonly Repository[],
): Map<string, number> {
  const counts = new Map<string, number>();
  for (const repository of repositories) {
    counts.set(repository.project_id, (counts.get(repository.project_id) ?? 0) + 1);
  }
  return counts;
}
