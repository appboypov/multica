import type { Issue, Release } from "../types";
import { lineStates, toLine, type ReleaseState } from "../releases/line";

/**
 * A release of the repository's line on a worklane's List, with the
 * worklane's issues in it. `done` counts the issues with a done date, which
 * the database keeps for every status of the done category (ADR-0015).
 */
export interface ReleaseListSection {
  kind: "release";
  release: Release;
  state: ReleaseState;
  /** The version was set by hand rather than by a bump. */
  setByHand: boolean;
  done: number;
  total: number;
  issues: Issue[];
}

/** The worklane's issues in no release of the line. */
export interface NoReleaseListSection {
  kind: "no_release";
  issues: Issue[];
}

export type WorklaneListSection = ReleaseListSection | NoReleaseListSection;

/**
 * The List of a worklane: one section per release of the line, in line
 * order, each with the worklane's issues in it, then "No release" while at
 * least one of the issues is in no release. `line` is the repository's
 * releases and `issues` the worklane's issues, both in the order to show; an
 * issue whose release is not on the line counts as in no release.
 */
export function listSections(
  line: readonly Release[],
  issues: readonly Issue[],
): WorklaneListSection[] {
  const ordered = [...line].sort((a, b) => a.position - b.position);
  const states = lineStates(toLine(ordered, null));
  const byRelease = new Map<string, Issue[]>(ordered.map((release) => [release.id, []]));
  const noRelease: Issue[] = [];
  for (const issue of issues) {
    const bucket = issue.release_id ? byRelease.get(issue.release_id) : undefined;
    if (bucket) bucket.push(issue);
    else noRelease.push(issue);
  }

  const sections: WorklaneListSection[] = ordered.map((release) => {
    const inRelease = byRelease.get(release.id)!;
    return {
      kind: "release",
      release,
      state: states.get(release.id) ?? "planned",
      setByHand: release.bump === null,
      done: inRelease.filter((issue) => issue.done_at != null).length,
      total: inRelease.length,
      issues: inRelease,
    };
  });
  if (noRelease.length > 0) sections.push({ kind: "no_release", issues: noRelease });
  return sections;
}
