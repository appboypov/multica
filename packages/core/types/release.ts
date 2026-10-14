export type ReleaseBump = "patch" | "minor" | "major";

/** Version has no leading `v`. Null bump means the version was set by hand. */
export interface Release {
  id: string;
  repository_id: string;
  position: number;
  version: string;
  bump: ReleaseBump | null;
  title: string;
  description: string;
  shipped_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface ListReleasesResponse {
  releases: Release[];
}

/** Every release write answers with the repository's whole line. */
export interface ReleaseLineResponse {
  releases: Release[];
}

/** Send either a bump or an explicit version, never both. */
export type PlanReleaseRequest = {
  title: string;
  description?: string;
} & (
  | { bump: ReleaseBump; version?: never }
  | { version: string; bump?: never }
);

export type EditReleaseRequest = PlanReleaseRequest;

export interface MoveReleaseRequest {
  direction: "earlier" | "later";
}

export interface ReleaseVersionChange {
  release_id: string;
  title: string;
  from: string;
  to: string;
}

export type ReleaseRefusalCode =
  | "title_required"
  | "version_or_bump"
  | "version_invalid"
  | "does_not_release"
  | "last_tag_unknown"
  | "first_release"
  | "last_release"
  | "line_not_increasing"
  | "unknown";

export interface ReleaseRefusal {
  code: ReleaseRefusalCode;
  error?: string;
  release_id?: string;
  version?: string;
  title?: string;
  before?: { version: string; title: string } | { last_tag: string };
}

export interface ReleaseActivityDetails {
  release_id: string;
  version: string;
  title: string;
}

export interface ReleaseEditedDetails extends ReleaseActivityDetails {
  changes: ReleaseVersionChange[];
}

export interface ReleaseMovedDetails extends ReleaseEditedDetails {
  direction: "earlier" | "later" | "unknown";
}

export interface ReleaseRemovedDetails extends ReleaseEditedDetails {}
