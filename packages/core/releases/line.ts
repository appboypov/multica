// The rules of a repository's release line (ADR-0010), ported from
// `server/internal/releaseline`. The client only previews with them; the
// server decides. Versions are written without the `v` everywhere except
// through `formatVersion`.

import type { Release, ReleaseBump, ReleaseVersionChange } from "../types";

export interface LineRelease {
  id: string;
  version: string;
  /** Null when the version was set by hand. */
  bump: ReleaseBump | null;
  title: string;
  shipped: boolean;
}

/** `lastTag` is the repository's last tag without the `v`, or "" when it has none. */
export interface Line {
  lastTag: string;
  releases: LineRelease[];
}

export type LineProblemCode =
  | "first_release"
  | "last_release"
  | "line_not_increasing"
  | "release_shipped"
  | "release_before_shipped"
  | "release_not_on_line";

/**
 * Why a change would break the line. For `line_not_increasing`, `release` is
 * the first release whose version is not higher than the one before it, and
 * `before` is that release, or null when the last tag stands before it.
 */
export interface LineProblem {
  code: LineProblemCode;
  release: LineRelease | null;
  before: LineRelease | null;
  lastTag: string;
}

export interface LinePreview {
  line: Line;
  changes: ReleaseVersionChange[];
  problem: LineProblem | null;
}

/** A release as the dialogs send it: a bump, or a version set by hand. */
export interface ReleaseDraft {
  title: string;
  bump: ReleaseBump | null;
  /** Without the `v`; read only when `bump` is null. */
  version: string;
}

export interface Version {
  major: number;
  minor: number;
  patch: number;
}

export type ReleaseState = "shipped" | "next" | "planned";

export const RELEASE_BUMPS: readonly ReleaseBump[] = ["patch", "minor", "major"];

const ZERO: Version = { major: 0, minor: 0, patch: 0 };

/** Accepts `v1.4.0` and `1.4.0`, with surrounding spaces. */
export function parseVersion(text: string): Version | null {
  const trimmed = text.trim();
  const bare = trimmed.startsWith("v") ? trimmed.slice(1) : trimmed;
  const parts = bare.split(".");
  if (parts.length !== 3) return null;
  const numbers: number[] = [];
  for (const part of parts) {
    if (!/^[0-9]+$/.test(part)) return null;
    numbers.push(Number(part));
  }
  return { major: numbers[0]!, minor: numbers[1]!, patch: numbers[2]! };
}

/** Writes a version without the `v`. */
export function versionString(v: Version): string {
  return `${v.major}.${v.minor}.${v.patch}`;
}

/** Writes a stored version, which has no `v`, as the UI shows it. */
export function formatVersion(version: string): string {
  return `v${version}`;
}

export function compareVersions(a: Version, b: Version): number {
  if (a.major !== b.major) return Math.sign(a.major - b.major);
  if (a.minor !== b.minor) return Math.sign(a.minor - b.minor);
  return Math.sign(a.patch - b.patch);
}

export function nextVersion(v: Version, bump: ReleaseBump): Version {
  switch (bump) {
    case "major":
      return { major: v.major + 1, minor: 0, patch: 0 };
    case "minor":
      return { major: v.major, minor: v.minor + 1, patch: 0 };
    default:
      return { major: v.major, minor: v.minor, patch: v.patch + 1 };
  }
}

function parseOrZero(text: string): Version {
  return parseVersion(text) ?? ZERO;
}

/** The repository's releases as a line, in position order. */
export function toLine(releases: readonly Release[], lastTag: string | null): Line {
  return {
    lastTag: lastTag ?? "",
    releases: [...releases]
      .sort((a, b) => a.position - b.position)
      .map((release) => ({
        id: release.id,
        version: release.version,
        bump: release.bump,
        title: release.title,
        shipped: release.shipped_at !== null,
      })),
  };
}

/**
 * Gives each unshipped release with a bump the version its bump gives after
 * the version before it. Before the first release stands the last tag, or
 * 0.0.0 when there is none. A release set by hand, and every shipped release,
 * keeps its version.
 */
export function renumber(line: Line): Line {
  let previous = parseOrZero(line.lastTag);
  const releases = line.releases.map((release) => {
    const next =
      !release.shipped && release.bump !== null
        ? { ...release, version: versionString(nextVersion(previous, release.bump)) }
        : release;
    previous = parseOrZero(next.version);
    return next;
  });
  return { lastTag: line.lastTag, releases };
}

/**
 * Finds the first release whose version is not higher than the one before
 * it. When the last tag is "", the first release has no lower bound.
 */
export function checkLine(line: Line): LineProblem | null {
  for (let i = 0; i < line.releases.length; i++) {
    const release = line.releases[i]!;
    const version = parseOrZero(release.version);
    if (i === 0) {
      if (line.lastTag === "") continue;
      if (compareVersions(version, parseOrZero(line.lastTag)) <= 0) {
        return { code: "line_not_increasing", release, before: null, lastTag: line.lastTag };
      }
      continue;
    }
    const before = line.releases[i - 1]!;
    if (compareVersions(version, parseOrZero(before.version)) <= 0) {
      return { code: "line_not_increasing", release, before, lastTag: line.lastTag };
    }
  }
  return null;
}

/** Every release of both lines whose version changed, in the order of `after`. */
export function lineChanges(before: Line, after: Line): ReleaseVersionChange[] {
  const old = new Map(before.releases.map((release) => [release.id, release.version]));
  const changes: ReleaseVersionChange[] = [];
  for (const release of after.releases) {
    const from = old.get(release.id);
    if (from === undefined || from === release.version) continue;
    changes.push({ release_id: release.id, title: release.title, from, to: release.version });
  }
  return changes;
}

function settle(before: Line, after: Line): LinePreview {
  const line = renumber(after);
  return { line, changes: lineChanges(before, line), problem: checkLine(line) };
}

function notOnLine(line: Line): LinePreview {
  return {
    line,
    changes: [],
    problem: { code: "release_not_on_line", release: null, before: null, lastTag: line.lastTag },
  };
}

/** The id the previews give a release that is not on the line yet. */
export const PLANNED_RELEASE_ID = "";

/** Adds the draft at the end of the line, then renumbers and checks. */
export function previewPlan(line: Line, draft: ReleaseDraft): LinePreview {
  const release: LineRelease = {
    id: PLANNED_RELEASE_ID,
    version: draft.bump === null ? draft.version : "",
    bump: draft.bump,
    title: draft.title,
    shipped: false,
  };
  return settle(line, { lastTag: line.lastTag, releases: [...line.releases, release] });
}

/**
 * Replaces the version, bump and title of release `id` in place, then
 * renumbers and checks. A new bump drops the version set by hand.
 */
export function previewEdit(line: Line, id: string, draft: ReleaseDraft): LinePreview {
  const index = line.releases.findIndex((release) => release.id === id);
  if (index < 0) return notOnLine(line);
  const current = line.releases[index]!;
  if (current.shipped && (draft.bump !== current.bump || (draft.bump === null && draft.version !== current.version))) {
    return { line, changes: [], problem: { code: "release_shipped", release: current, before: null, lastTag: line.lastTag } };
  }
  const releases = line.releases.map((release, i) =>
    i === index
      ? {
          ...release,
          version: current.shipped ? current.version : draft.bump === null ? draft.version : release.version,
          bump: draft.bump,
          title: draft.title,
        }
      : release,
  );
  return settle(line, { lastTag: line.lastTag, releases });
}

/**
 * Swaps release `id` with its neighbour in `direction`, then renumbers and
 * checks. It refuses `first_release` and `last_release` first.
 */
export function previewMove(line: Line, id: string, direction: "earlier" | "later"): LinePreview {
  const index = line.releases.findIndex((release) => release.id === id);
  if (index < 0) return notOnLine(line);
  const release = line.releases[index]!;
  if (release.shipped) {
    return { line, changes: [], problem: { code: "release_shipped", release, before: null, lastTag: line.lastTag } };
  }
  if (direction === "earlier" && index === 0) {
    return { line, changes: [], problem: { code: "first_release", release, before: null, lastTag: line.lastTag } };
  }
  if (direction === "later" && index === line.releases.length - 1) {
    return { line, changes: [], problem: { code: "last_release", release, before: null, lastTag: line.lastTag } };
  }
  const other = direction === "earlier" ? index - 1 : index + 1;
  if (line.releases[other]!.shipped) {
    return { line, changes: [], problem: { code: "release_before_shipped", release, before: null, lastTag: line.lastTag } };
  }
  const releases = [...line.releases];
  releases[index] = releases[other]!;
  releases[other] = release;
  return settle(line, { lastTag: line.lastTag, releases });
}

/** Takes release `id` off the line, then renumbers and checks. */
export function previewRemove(line: Line, id: string): LinePreview {
  const index = line.releases.findIndex((release) => release.id === id);
  if (index < 0) return notOnLine(line);
  const release = line.releases[index]!;
  if (release.shipped) {
    return { line, changes: [], problem: { code: "release_shipped", release, before: null, lastTag: line.lastTag } };
  }
  return settle(line, {
    lastTag: line.lastTag,
    releases: line.releases.filter((release) => release.id !== id),
  });
}

/**
 * What stands before the release at `index`: the release before it, the last
 * tag, or nothing on an empty line without a tag.
 */
export type VersionBefore =
  | { kind: "release"; release: LineRelease }
  | { kind: "tag"; tag: string }
  | { kind: "none" };

export function versionBefore(line: Line, index: number): VersionBefore {
  const release = index > 0 ? line.releases[index - 1] : undefined;
  if (release) return { kind: "release", release };
  if (line.lastTag !== "") return { kind: "tag", tag: line.lastTag };
  return { kind: "none" };
}

export interface BumpOption {
  bump: ReleaseBump;
  /** Without the `v`. */
  version: string;
}

/** Patch, Minor and Major for the release at `index`, each with the version it gives. */
export function bumpOptions(line: Line, index: number): BumpOption[] {
  const before = versionBefore(line, index);
  const base =
    before.kind === "release"
      ? parseOrZero(before.release.version)
      : before.kind === "tag"
        ? parseOrZero(before.tag)
        : ZERO;
  return RELEASE_BUMPS.map((bump) => ({ bump, version: versionString(nextVersion(base, bump)) }));
}

/** Shipped releases are shipped, the first unshipped one is next, the rest are planned. */
export function lineStates(line: Line): Map<string, ReleaseState> {
  const states = new Map<string, ReleaseState>();
  let nextTaken = false;
  for (const release of line.releases) {
    if (release.shipped) {
      states.set(release.id, "shipped");
    } else if (!nextTaken) {
      states.set(release.id, "next");
      nextTaken = true;
    } else {
      states.set(release.id, "planned");
    }
  }
  return states;
}

/** The last shipped release of the line, or null while none has shipped. */
export function lastShipped(line: Line): LineRelease | null {
  let last: LineRelease | null = null;
  for (const release of line.releases) if (release.shipped) last = release;
  return last;
}

/** The first unshipped release of the line, or null when none is planned. */
export function nextRelease(line: Line): LineRelease | null {
  return line.releases.find((release) => !release.shipped) ?? null;
}
