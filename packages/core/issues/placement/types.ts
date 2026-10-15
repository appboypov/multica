import type { ReleaseProgressRow } from "../../types/issue-template";
import type { ReleaseMilestone } from "../../types/release";

/** Every issue of the release, including closed issues whose worklane was removed. */
export interface ReleaseTotal {
  release_id: string;
  done: number;
  total: number;
  milestones: ReleaseMilestone[];
}

export interface ReleaseProgressResponse {
  rows: ReleaseProgressRow[];
  releases: ReleaseTotal[];
}
