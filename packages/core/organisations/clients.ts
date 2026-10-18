import type { MemberWithUser, Project } from "../types";

/**
 * The organisation of one or more issues: the organisation their projects
 * share. Null when an issue has no project, a project has no organisation or
 * is unknown, or the issues span more than one organisation.
 */
export function issueOrganisationId(
  projectIds: readonly (string | null | undefined)[],
  projects: readonly Pick<Project, "id" | "organisation_id">[],
): string | null {
  if (projectIds.length === 0) return null;
  let organisationId: string | null = null;
  for (const projectId of projectIds) {
    const orgId = projects.find((p) => p.id === projectId)?.organisation_id ?? null;
    if (!orgId) return null;
    if (organisationId && organisationId !== orgId) return null;
    organisationId = orgId;
  }
  return organisationId;
}

/** The client members of an organisation. */
export function clientsOfOrganisation(
  members: readonly MemberWithUser[],
  organisationId: string | null,
): MemberWithUser[] {
  if (!organisationId) return [];
  return members.filter(
    (m) => m.role === "client" && m.organisation_ids.includes(organisationId),
  );
}

/**
 * A client's organisation set with one organisation added or removed. The
 * last organisation stays: a client always holds at least one.
 */
export function toggleClientOrganisation(
  organisationIds: readonly string[],
  organisationId: string,
): string[] {
  if (!organisationIds.includes(organisationId)) {
    return [...organisationIds, organisationId];
  }
  if (organisationIds.length === 1) return [...organisationIds];
  return organisationIds.filter((id) => id !== organisationId);
}
