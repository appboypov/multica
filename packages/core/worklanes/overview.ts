import { buildWorkTree, type WorkTree, type WorkTreeOrder } from "../projects";
import type { Organisation, Project, Repository, Worklane } from "../types";
import { sortWorklanes } from "./counts";

export interface WorklaneOverviewRepository {
  repository: Repository;
  /** `main` first, then the others in the order they were made. */
  worklanes: Worklane[];
}

export type WorklaneOverview = WorkTree<WorklaneOverviewRepository>;

/**
 * The Worklanes page's tree: every worklane under its repository, project and
 * organisation (ADR-0016). A repository without worklanes is left out.
 */
export function worklaneOverview({
  organisations,
  projects,
  repositories,
  worklanes,
  order,
}: {
  organisations: readonly Organisation[];
  projects: readonly Project[];
  repositories: readonly Repository[];
  worklanes: readonly Worklane[];
  order: WorkTreeOrder;
}): WorklaneOverview {
  const byRepository = new Map<string, Worklane[]>();
  for (const worklane of worklanes) {
    const list = byRepository.get(worklane.repository_id);
    if (list) list.push(worklane);
    else byRepository.set(worklane.repository_id, [worklane]);
  }
  return buildWorkTree<WorklaneOverviewRepository>({
    organisations,
    projects,
    repositories,
    order,
    countOf: (entry) => entry.worklanes.length,
    rowsOf: (repository) => {
      const list = byRepository.get(repository.id);
      return list ? { repository, worklanes: sortWorklanes(list) } : null;
    },
  });
}
