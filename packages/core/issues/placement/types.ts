import type { ReleaseProgressRow } from "../../types/issue-template";

/** Every issue of the release, including closed issues whose worklane was removed. */
export interface ReleaseTotal {
  release_id: string;
  done: number;
  total: number;
}

export interface ReleaseProgressResponse {
  rows: ReleaseProgressRow[];
  releases: ReleaseTotal[];
}
