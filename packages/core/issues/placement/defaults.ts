import type { IssueStatus, Release, Repository, Worklane } from "../../types";

/**
 * Where New issue opens fills in the worklane and its release (design.md
 * decision 11, the `new-issue-placement` spec delta). Pure functions over the
 * cached worklanes, repositories and releases, so the dialog and its tests
 * share them.
 */

/** The first unshipped release of a line, the line in position order. */
export function nextRelease(line: readonly Release[]): Release | null {
  let next: Release | null = null;
  for (const release of line) {
    if (release.shipped_at !== null) continue;
    if (next === null || release.position < next.position) next = release;
  }
  return next;
}

/** The unshipped releases of a repository's line, in their order. */
export function unshippedReleases(releases: readonly Release[], repositoryId: string): Release[] {
  return releases
    .filter((release) => release.repository_id === repositoryId && release.shipped_at === null)
    .sort((a, b) => a.position - b.position);
}

/** Where New issue was opened from. */
export type NewIssueOrigin =
  | { worklaneId: string; releaseId?: string | null }
  | { projectId: string }
  | Record<string, never>;

export interface PlacementData {
  worklanes: readonly Worklane[];
  repositories: readonly Repository[];
  releases: readonly Release[];
}

export interface Placement {
  worklaneId: string | null;
  releaseId: string | null;
}

const NO_PLACEMENT: Placement = { worklaneId: null, releaseId: null };

/** The release a picked worklane fills in: its repository's next release. */
export function releaseForWorklane(worklaneId: string | null, data: PlacementData): string | null {
  if (!worklaneId) return null;
  const worklane = data.worklanes.find((w) => w.id === worklaneId);
  if (!worklane) return null;
  return nextRelease(unshippedReleases(data.releases, worklane.repository_id))?.id ?? null;
}

/**
 * The release an issue keeps when its page picks another worklane: its own
 * release when the new worklane is of the same repository as the old one,
 * else none (the `issue-page-worklane-and-release` spec delta).
 */
export function releaseKeptOnWorklanePick(
  from: { worklaneId: string | null; releaseId: string | null },
  toWorklaneId: string | null,
  data: PlacementData,
): string | null {
  if (!from.worklaneId || !from.releaseId || !toWorklaneId) return null;
  const repositoryOf = (id: string) => data.worklanes.find((w) => w.id === id)?.repository_id;
  const repository = repositoryOf(from.worklaneId);
  return repository !== undefined && repository === repositoryOf(toWorklaneId) ? from.releaseId : null;
}

/**
 * The worklane and release New issue fills in. From a worklane: that worklane
 * and its repository's next release, or the release it was opened in (+ Add
 * issue to release). From a project: `main` of its first repository by
 * `full_name` and that repository's next release. From elsewhere: none.
 */
export function openedFrom(origin: NewIssueOrigin, data: PlacementData): Placement {
  if ("worklaneId" in origin) {
    const releaseId =
      origin.releaseId !== undefined ? origin.releaseId : releaseForWorklane(origin.worklaneId, data);
    return { worklaneId: origin.worklaneId, releaseId };
  }
  if ("projectId" in origin) {
    const first = data.repositories
      .filter((repository) => repository.project_id === origin.projectId)
      .sort((a, b) => a.full_name.localeCompare(b.full_name))[0];
    if (!first) return NO_PLACEMENT;
    const main = data.worklanes.find((w) => w.repository_id === first.id && w.is_main);
    if (!main) return NO_PLACEMENT;
    return { worklaneId: main.id, releaseId: releaseForWorklane(main.id, data) };
  }
  return NO_PLACEMENT;
}

/** The status Brian picked, otherwise Todo with a release and Backlog without. */
export function defaultStatus(releaseId: string | null, picked: IssueStatus | null): IssueStatus {
  if (picked) return picked;
  return releaseId ? "todo" : "backlog";
}
