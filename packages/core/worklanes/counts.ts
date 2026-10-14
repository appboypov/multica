import type { Project, Repository, Worklane } from "../types";

/** A repository's worklanes in the card's order: `main` first, then by position. */
export function sortWorklanes<T extends Pick<Worklane, "is_main" | "position" | "name">>(
  worklanes: readonly T[],
): T[] {
  return [...worklanes].sort((a, b) => {
    if (a.is_main !== b.is_main) return a.is_main ? -1 : 1;
    if (a.position !== b.position) return a.position - b.position;
    return a.name.localeCompare(b.name);
  });
}

export interface WorklaneCounts {
  byRepository: Map<string, number>;
  byProject: Map<string, number>;
  byOrganisation: Map<string, number>;
}

/**
 * The number of worklanes, `main` included, each repository, project and
 * organisation holds. A key that holds none is absent: read it with `?? 0`.
 */
export function countWorklanes(
  worklanes: readonly Pick<Worklane, "repository_id">[],
  repositories: readonly Pick<Repository, "id" | "project_id">[],
  projects: readonly Pick<Project, "id" | "organisation_id">[] = [],
): WorklaneCounts {
  const byRepository = new Map<string, number>();
  for (const worklane of worklanes) {
    byRepository.set(worklane.repository_id, (byRepository.get(worklane.repository_id) ?? 0) + 1);
  }
  const byProject = new Map<string, number>();
  for (const repository of repositories) {
    const count = byRepository.get(repository.id);
    if (!count) continue;
    byProject.set(repository.project_id, (byProject.get(repository.project_id) ?? 0) + count);
  }
  const byOrganisation = new Map<string, number>();
  for (const project of projects) {
    const count = byProject.get(project.id);
    if (!count || !project.organisation_id) continue;
    byOrganisation.set(
      project.organisation_id,
      (byOrganisation.get(project.organisation_id) ?? 0) + count,
    );
  }
  return { byRepository, byProject, byOrganisation };
}
