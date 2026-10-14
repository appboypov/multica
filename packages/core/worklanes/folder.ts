import type { Worklane } from "../types";

/** One folder name: lowercase letters, digits and dashes, starting with a letter or digit. */
export const WORKLANE_FOLDER_PATTERN = /^[a-z0-9][a-z0-9-]*$/;

/**
 * The folder New worklane fills in from a name: lowercase, each run of other
 * characters a dash, without a dash at the start or end.
 */
export function folderFromName(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export type WorklaneFolderProblem =
  | { kind: "invalid" }
  | { kind: "in_use"; worklaneName: string };

/**
 * The rule a folder breaks among the repository's worklanes, or null when it
 * is free. An empty folder is `invalid`.
 */
export function folderProblem(
  folder: string,
  worklanes: readonly Pick<Worklane, "name" | "folder">[],
): WorklaneFolderProblem | null {
  if (!WORKLANE_FOLDER_PATTERN.test(folder)) return { kind: "invalid" };
  const holder = worklanes.find((worklane) => worklane.folder === folder);
  return holder ? { kind: "in_use", worklaneName: holder.name } : null;
}

/**
 * Where a new worklane's worktree goes, the server's rule: under the
 * organisation key, or directly under Worktrees without an organisation.
 * `fullName` is `owner/name`; only `name` is a folder.
 */
export function worklanePath(
  organisationKey: string | null | undefined,
  fullName: string,
  folder: string,
): string {
  const repo = fullName.slice(fullName.lastIndexOf("/") + 1);
  return organisationKey
    ? `~/Worktrees/${organisationKey}/${repo}/${folder}`
    : `~/Worktrees/${repo}/${folder}`;
}
