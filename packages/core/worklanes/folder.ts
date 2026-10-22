import type { Worklane, Workspace } from "../types";

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

/** The work root of a workspace without `work_root` in its settings. */
export const DEFAULT_WORK_ROOT = "~/hq";

/**
 * The folder that holds every organisation's folder, read as the server
 * reads it: the workspace's `work_root` setting trimmed and cleaned (repeated
 * slashes, `.` segments and a trailing slash dropped). It is `~/hq` when the
 * setting is absent, not a string, blank, not absolute nor under `~`, or has a
 * `..` segment.
 */
export function workspaceWorkRoot(
  workspace: Pick<Workspace, "settings"> | null | undefined,
): string {
  const value: unknown = workspace?.settings?.work_root;
  if (typeof value !== "string") return DEFAULT_WORK_ROOT;
  const raw = value.trim();
  const absolute = raw.startsWith("/");
  if (!(absolute || raw === "~" || raw.startsWith("~/"))) return DEFAULT_WORK_ROOT;
  const segments = raw.split("/").filter((segment) => segment !== "" && segment !== ".");
  if (segments.includes("..")) return DEFAULT_WORK_ROOT;
  return (absolute ? "/" : "") + segments.join("/");
}

/** Where a project's folder lies: the work root, its organisation's key and its slug. */
export interface ProjectPlace {
  workRoot: string;
  organisationKey: string | null | undefined;
  projectSlug: string | null | undefined;
}

/**
 * `{root}/{org}/{project}/{area}/{repo}`, with `repo` the `name` of
 * `owner/name`, or null while the project lacks an organisation or a slug.
 */
function repositoryFolder(
  { workRoot, organisationKey, projectSlug }: ProjectPlace,
  area: "repos" | "worktrees",
  fullName: string,
): string | null {
  if (!organisationKey || !projectSlug) return null;
  const repo = fullName.slice(fullName.lastIndexOf("/") + 1);
  return `${workRoot}/${organisationKey}/${projectSlug}/${area}/${repo}`;
}

/**
 * The checkout Add repository fills in, the server's rule:
 * `{root}/{org}/{project}/repos/{repo}`. Null while the project lacks an
 * organisation or a slug, which the server requires for a repository.
 */
export function checkoutPath(place: ProjectPlace, fullName: string): string | null {
  return repositoryFolder(place, "repos", fullName);
}

/**
 * Where a new worklane's worktree goes, the daemon's rule:
 * `{root}/{org}/{project}/worktrees/{repo}/{folder}`. Null while the project
 * lacks an organisation or a slug.
 */
export function worklanePath(
  place: ProjectPlace,
  fullName: string,
  folder: string,
): string | null {
  const worktrees = repositoryFolder(place, "worktrees", fullName);
  return worktrees && `${worktrees}/${folder}`;
}
