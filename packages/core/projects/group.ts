import type { Organisation, Project } from "../types";

export interface ProjectOrganisationGroup {
  organisation: Organisation | null;
  projects: Project[];
}

export function groupProjectsByOrganisation(
  projects: readonly Project[],
  organisations: readonly Organisation[],
): ProjectOrganisationGroup[] {
  const byId = new Map(organisations.map((organisation) => [organisation.id, organisation]));
  const groups = new Map<string | null, ProjectOrganisationGroup>();
  for (const project of projects) {
    const organisation = project.organisation_id ? byId.get(project.organisation_id) ?? null : null;
    const id = organisation?.id ?? null;
    let group = groups.get(id);
    if (!group) {
      group = { organisation, projects: [] };
      groups.set(id, group);
    }
    group.projects.push(project);
  }
  return [...groups.values()].sort((a, b) => {
    if (!a.organisation) return b.organisation ? 1 : 0;
    if (!b.organisation) return -1;
    return a.organisation.name.localeCompare(b.organisation.name, undefined, { sensitivity: "base" });
  });
}
