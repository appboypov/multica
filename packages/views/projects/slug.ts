import type { Project } from "@multica/core/types";
import { ORGANISATION_KEY_PATTERN } from "../organisations/validation";

export type ProjectSlugProblem = "required" | "format" | "taken";

/**
 * What stops `slug` from naming a project's folder under its organisation's
 * folder: empty while the project holds repositories, not a folder name (the
 * organisation key's pattern), or the slug of another project of the same
 * organisation. An empty slug clears it. The server applies the same rules.
 */
export function projectSlugProblem(
  slug: string,
  project: Pick<Project, "id" | "organisation_id">,
  projects: readonly Pick<Project, "id" | "organisation_id" | "slug">[],
  hasRepositories: boolean,
): ProjectSlugProblem | null {
  if (!slug) return hasRepositories ? "required" : null;
  if (!ORGANISATION_KEY_PATTERN.test(slug)) return "format";
  const taken =
    project.organisation_id !== null &&
    projects.some(
      (other) =>
        other.id !== project.id &&
        other.organisation_id === project.organisation_id &&
        other.slug === slug,
    );
  return taken ? "taken" : null;
}
