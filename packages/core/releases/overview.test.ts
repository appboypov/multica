import { describe, expect, it } from "vitest";
import { EMPTY_RELEASE } from "../api/schemas";
import type { Release, ReleaseProgressResponse, Worklane } from "../types";
import { releaseTimelineTarget } from "./overview";

const worklane = (overrides: Partial<Worklane> & Pick<Worklane, "id" | "name">): Worklane => ({
  workspace_id: "ws-1",
  repository_id: "repo-1",
  folder: null,
  daemon_id: null,
  path: null,
  is_main: false,
  position: 0,
  state: "free",
  branch: null,
  active_issue: null,
  next_issue: null,
  created_at: "",
  updated_at: "",
  ...overrides,
});

const main = worklane({ id: "w-main", name: "main", is_main: true });
const accounts = worklane({ id: "w-accounts", name: "Fan accounts and push", position: 1 });
const profile = worklane({ id: "w-profile", name: "Profile photos", position: 2 });
const elsewhere = worklane({ id: "w-other", name: "Other repo", repository_id: "repo-2", position: 0 });
const release: Release = { ...EMPTY_RELEASE, id: "r-110", repository_id: "repo-1", version: "1.1.0", title: "Account and profile" };

const progress = (rows: Array<[worklaneId: string, releaseId: string, total: number]>): ReleaseProgressResponse => ({
  rows: rows.map(([worklane_id, release_id, total]) => ({ worklane_id, release_id, done: 0, total })),
  releases: [],
});

describe("releaseTimelineTarget", () => {
  it("opens the first worklane of the repository, in its order, that holds issues in the release", () => {
    expect(
      releaseTimelineTarget({
        release,
        // Out of order on purpose: the repository's worklane order decides.
        worklanes: [profile, elsewhere, accounts, main],
        progress: progress([
          ["w-profile", "r-110", 2],
          ["w-accounts", "r-110", 1],
          ["w-other", "r-110", 4],
        ]),
      })?.id,
    ).toBe("w-accounts");
  });

  it("falls back to main when no worklane holds issues in the release", () => {
    expect(
      releaseTimelineTarget({
        release,
        worklanes: [accounts, main],
        progress: progress([
          ["w-accounts", "r-110", 0],
          ["w-accounts", "r-100", 3],
        ]),
      })?.id,
    ).toBe("w-main");
  });

  it("has no target while the repository's worklanes are unknown", () => {
    expect(releaseTimelineTarget({ release, worklanes: [elsewhere], progress: progress([]) })).toBeNull();
  });
});
