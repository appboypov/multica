import type { Organisation } from "@multica/core/types";

/**
 * An organisation's key names its worktree folder: lowercase letters, digits
 * and dashes, starting with a letter or digit. The server applies the same
 * rule; checking it here gives the message at once.
 */
export const ORGANISATION_KEY_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

export type OrganisationNameProblem = "required" | "taken";
export type OrganisationKeyProblem = "required" | "format" | "taken";

/**
 * What stops `name` from naming an organisation, checked against the loaded
 * list: empty, or another organisation's name ignoring case. `selfId` is the
 * organisation being edited, which may keep its own name.
 */
export function organisationNameProblem(
  name: string,
  organisations: readonly Organisation[],
  selfId?: string,
): OrganisationNameProblem | null {
  const wanted = name.trim().toLowerCase();
  if (!wanted) return "required";
  const taken = organisations.some(
    (organisation) =>
      organisation.id !== selfId && organisation.name.trim().toLowerCase() === wanted,
  );
  return taken ? "taken" : null;
}

/**
 * What stops `key` from keying an organisation: empty, not a folder name
 * (`ORGANISATION_KEY_PATTERN`), or another organisation's key. `selfId` is
 * the organisation being edited, which may keep its own key.
 */
export function organisationKeyProblem(
  key: string,
  organisations: readonly Organisation[],
  selfId?: string,
): OrganisationKeyProblem | null {
  if (!key) return "required";
  if (!ORGANISATION_KEY_PATTERN.test(key)) return "format";
  const taken = organisations.some(
    (organisation) => organisation.id !== selfId && organisation.key === key,
  );
  return taken ? "taken" : null;
}
