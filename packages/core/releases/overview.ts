import { buildWorkTree, type WorkTree, type WorkTreeOrder } from "../projects";
import type {
  Organisation,
  Project,
  Release,
  ReleaseProgressResponse,
  Repository,
  Worklane,
} from "../types";
import { sortWorklanes } from "../worklanes";
import { lineStates, toLine } from "./line";
import { selectLine } from "./queries";

export interface ReleaseOverviewRow {
  release: Release;
  state: "shipped" | "next" | "planned";
  /** The version was set by hand rather than by a bump. */
  setByHand: boolean;
  /** Over every issue in the release, also a closed one whose worklane was removed. */
  done: number;
  total: number;
  /** The repository's worklanes that hold issues in the release, in their order. */
  worklanes: Worklane[];
  /** The worklane whose Timeline the row opens: the first of `worklanes`, else `main`. */
  target: Worklane | null;
}

export interface ReleaseOverviewRepository {
  repository: Repository;
  /** Set to "This repository does not release": kept without rows. */
  doesNotRelease: boolean;
  releases: ReleaseOverviewRow[];
}

export type ReleaseOverview = WorkTree<ReleaseOverviewRepository>;

/**
 * The Releases page's tree: every repository's releases in line order,
 * under their project and organisation (ADR-0016). A repository without
 * releases is left out; one that does not release keeps `doesNotRelease`.
 */
export function releaseOverview({
  organisations,
  projects,
  repositories,
  worklanes,
  releases,
  progress,
  order,
}: {
  organisations: readonly Organisation[];
  projects: readonly Project[];
  repositories: readonly Repository[];
  worklanes: readonly Worklane[];
  releases: readonly Release[];
  progress: ReleaseProgressResponse;
  order: WorkTreeOrder;
}): ReleaseOverview {
  const totals = new Map(progress.releases.map((entry) => [entry.release_id, entry]));
  const heldBy = new Set(
    progress.rows.filter((row) => row.total > 0).map((row) => `${row.worklane_id}:${row.release_id}`),
  );
  const worklanesByRepository = new Map<string, Worklane[]>();
  for (const worklane of worklanes) {
    const list = worklanesByRepository.get(worklane.repository_id);
    if (list) list.push(worklane);
    else worklanesByRepository.set(worklane.repository_id, [worklane]);
  }

  return buildWorkTree<ReleaseOverviewRepository>({
    organisations,
    projects,
    repositories,
    order,
    countOf: (entry) => entry.releases.length,
    rowsOf: (repository) => {
      if (repository.version_policy === "none") {
        return { repository, doesNotRelease: true, releases: [] };
      }
      const line = selectLine(releases, repository.id);
      const states = lineStates(toLine(line, repository.last_tag));
      const repositoryWorklanes = sortWorklanes(worklanesByRepository.get(repository.id) ?? []);
      const main = repositoryWorklanes.find((worklane) => worklane.is_main) ?? null;
      const rows: ReleaseOverviewRow[] = [];
      for (const release of line) {
        const state = states.get(release.id);
        if (!state) continue;
        const holding = repositoryWorklanes.filter((worklane) =>
          heldBy.has(`${worklane.id}:${release.id}`),
        );
        const counts = totals.get(release.id);
        rows.push({
          release,
          state,
          setByHand: release.bump === null && state !== "shipped",
          done: counts?.done ?? 0,
          total: counts?.total ?? 0,
          worklanes: holding,
          target: holding[0] ?? main,
        });
      }
      return rows.length > 0 ? { repository, doesNotRelease: false, releases: rows } : null;
    },
  });
}
