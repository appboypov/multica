import type { Organisation, Project, Repository } from "../types";
import { groupProjectsByOrganisation } from "./group";
import { sortProjects } from "./sort";
import type { ProjectSortDirection, ProjectSortField } from "./stores/view-store";

/** The project view store's sort, which Worklanes and Releases share with Projects. */
export interface WorkTreeOrder {
  field: ProjectSortField;
  direction: ProjectSortDirection;
}

export interface WorkTreeProject<R> {
  project: Project;
  /** The repositories kept under the project. */
  repositoryCount: number;
  /** The rows kept under the project. */
  count: number;
  repositories: R[];
}

export interface WorkTreeGroup<R> {
  /** Null for the projects without an organisation, listed last. */
  organisation: Organisation | null;
  count: number;
  projects: WorkTreeProject<R>[];
}

export interface WorkTree<R> {
  groups: WorkTreeGroup<R>[];
  /** Every row the tree keeps. */
  total: number;
}

/**
 * Builds the organisation, project and repository tree of Worklanes and
 * Releases: organisations and projects in Projects' order, repositories by
 * `full_name`. `rowsOf` gives a repository's entry, or null to leave it out;
 * a project or organisation left with nothing is left out too.
 */
export function buildWorkTree<R>({
  organisations,
  projects,
  repositories,
  order,
  rowsOf,
  countOf,
}: {
  organisations: readonly Organisation[];
  projects: readonly Project[];
  repositories: readonly Repository[];
  order: WorkTreeOrder;
  rowsOf: (repository: Repository) => R | null;
  countOf: (entry: R) => number;
}): WorkTree<R> {
  const repositoriesByProject = new Map<string, Repository[]>();
  for (const repository of repositories) {
    const list = repositoriesByProject.get(repository.project_id);
    if (list) list.push(repository);
    else repositoriesByProject.set(repository.project_id, [repository]);
  }

  const groups: WorkTreeGroup<R>[] = [];
  let total = 0;
  const sorted = sortProjects(projects, order.field, order.direction);
  for (const group of groupProjectsByOrganisation(sorted, organisations)) {
    const treeProjects: WorkTreeProject<R>[] = [];
    let groupCount = 0;
    for (const project of group.projects) {
      const entries: R[] = [];
      let projectCount = 0;
      const projectRepositories = [...(repositoriesByProject.get(project.id) ?? [])].sort((a, b) =>
        a.full_name.localeCompare(b.full_name),
      );
      for (const repository of projectRepositories) {
        const entry = rowsOf(repository);
        if (!entry) continue;
        entries.push(entry);
        projectCount += countOf(entry);
      }
      if (entries.length === 0) continue;
      treeProjects.push({
        project,
        repositoryCount: entries.length,
        count: projectCount,
        repositories: entries,
      });
      groupCount += projectCount;
    }
    if (treeProjects.length === 0) continue;
    groups.push({ organisation: group.organisation, count: groupCount, projects: treeProjects });
    total += groupCount;
  }
  return { groups, total };
}
