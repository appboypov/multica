import { describe, expect, it } from "vitest";
import type { Release, Worklane } from "../../types";
import { releaseKeptOnWorklanePick, type PlacementData } from "./defaults";

const worklane = (id: string, repository_id: string) => ({ id, repository_id }) as Worklane;
const release = (id: string, repository_id: string, position: number) =>
  ({ id, repository_id, position, shipped_at: null }) as Release;

// Rebl's v1.0.0 is its next release; REBL-21 sits in v1.1.0.
const data: PlacementData = {
  worklanes: [worklane("fan", "rebl"), worklane("agenda", "rebl"), worklane("admin-main", "rebl-admin")],
  repositories: [],
  releases: [release("v1.0.0", "rebl", 1), release("v1.1.0", "rebl", 2), release("admin-v1", "rebl-admin", 1)],
};

describe("releaseKeptOnWorklanePick", () => {
  const rebl21 = { worklaneId: "fan", releaseId: "v1.1.0" };

  it("keeps the issue's own release, not the next one, for a worklane of the same repository", () => {
    expect(releaseKeptOnWorklanePick(rebl21, "agenda", data)).toBe("v1.1.0");
  });

  it("sets no release for a worklane of another repository, even when it has releases", () => {
    expect(releaseKeptOnWorklanePick(rebl21, "admin-main", data)).toBeNull();
  });

  it("sets no release for No worklane and for an issue that had no worklane", () => {
    expect(releaseKeptOnWorklanePick(rebl21, null, data)).toBeNull();
    expect(releaseKeptOnWorklanePick({ worklaneId: null, releaseId: null }, "fan", data)).toBeNull();
  });
});
