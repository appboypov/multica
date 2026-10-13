/**
 * Stable cancelled-issue demotion for aggregated search results.
 * Projects retain their relevance order regardless of lifecycle status.
 * Direct issue hits are exempt, matching server search.
 */

import type { SearchIssueResult, SearchProjectResult } from "../types/api";
import { issueBehavesAs } from "../issues/status-category";

/**
 * Mirrors the server's identifier pattern (parseQueryNumber in
 * server/internal/handler/issue.go): "MUL-123" or a bare "123".
 */
const IDENTIFIER_NUMBER_RE = /^[a-z]+-(\d+)$/i;

/** Extracts the issue number a query targets, or null when it targets none. */
export function parseSearchQueryNumber(query: string): number | null {
  const q = query.trim();
  const m = IDENTIFIER_NUMBER_RE.exec(q);
  if (m) {
    const n = Number.parseInt(m[1]!, 10);
    if (Number.isInteger(n) && n > 0) return n;
  }
  if (/^\d+$/.test(q)) {
    const n = Number.parseInt(q, 10);
    if (Number.isInteger(n) && n > 0) return n;
  }
  return null;
}

function isExactTitle(title: string | null | undefined, query: string): boolean {
  if (!title) return false;
  const q = query.trim();
  if (!q) return false;
  return title.trim().toLowerCase() === q.toLowerCase();
}

/**
 * A query targets this specific issue: exact identifier / bare number, or the
 * full title.
 *
 * Every field is optional so callers holding only part of a row can share the
 * one rule — the @mention picker, for instance, carries the identifier and the
 * title but no `number`, which is then derived from the identifier.
 *
 * Note: the server compares the title against an escapeLike'd parameter, so a
 * title containing `_` or `%` never counts as a direct hit there (a pre-existing
 * tier-1 escaping quirk, see buildSearchQuery). Client-side we compare the raw
 * strings, which can only ever *keep* such an issue in the live partition —
 * never demote something the server kept up — so the two stay compatible.
 */
export function isIssueDirectHit(
  issue: {
    title?: string | null;
    number?: number | null;
    identifier?: string | null;
  },
  query: string,
): boolean {
  const targetNumber = parseSearchQueryNumber(query);
  if (targetNumber !== null) {
    const issueNumber =
      issue.number ??
      (issue.identifier ? parseSearchQueryNumber(issue.identifier) : null);
    if (issueNumber === targetNumber) return true;
  }
  if (
    issue.identifier &&
    issue.identifier.trim().toLowerCase() === query.trim().toLowerCase()
  ) {
    return true;
  }
  return isExactTitle(issue.title, query);
}

/** A query targets this specific project: its full title. */
export function isProjectDirectHit(
  project: { title?: string | null },
  query: string,
): boolean {
  return isExactTitle(project.title, query);
}

export interface CancelledPartition<T> {
  /** Everything that is not demoted, in its original relative order. */
  live: T[];
  /** Demoted cancelled rows, in their original relative order. */
  cancelled: T[];
}

/**
 * Stable partition on an arbitrary predicate. Split out so the caller decides
 * what "cancelled and not exempt" means for its own row shape, while the
 * order-preserving guarantee lives in one place.
 */
export function partitionStable<T>(
  items: readonly T[],
  demote: (item: T) => boolean,
): CancelledPartition<T> {
  const live: T[] = [];
  const cancelled: T[] = [];
  for (const item of items) {
    if (demote(item)) cancelled.push(item);
    else live.push(item);
  }
  return { live, cancelled };
}

export interface AggregatedSearchPartition {
  /** All projects, in their original relevance order. */
  liveProjects: SearchProjectResult[];
  /** Issues to render after the live projects, minus the demoted ones. */
  liveIssues: SearchIssueResult[];
  /** Project results have no cancellation bucket; shared consumers retain this shape. */
  cancelledProjects: SearchProjectResult[];
  cancelledIssues: SearchIssueResult[];
  /** True when anything was demoted; lets callers skip an empty section. */
  hasCancelled: boolean;
}

/**
 * Partitions a global-search result pair (issues + projects) for rendering.
 *
 * Render projects, live issues, then cancelled issues. Truncate only after
 * concatenating in that order so cancelled issues give up their slots.
 */
export function partitionAggregatedSearchResults({
  issues,
  projects,
  query,
}: {
  issues: readonly SearchIssueResult[];
  projects: readonly SearchProjectResult[];
  query: string;
}): AggregatedSearchPartition {
  const issueParts = partitionStable(
    issues,
    // By CATEGORY: a custom status in the cancelled category is cancelled
    // work and has to sink the same way. (MUL-6243)
    (issue) => issueBehavesAs(issue, "closed") && !isIssueDirectHit(issue, query),
  );

  return {
    liveProjects: [...projects],
    liveIssues: issueParts.live,
    cancelledProjects: [],
    cancelledIssues: issueParts.cancelled,
    hasCancelled: issueParts.cancelled.length > 0,
  };
}
