"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { worklaneIssuesOptions } from "@multica/core/issues/placement";
import { releaseListOptions, selectLine } from "@multica/core/releases";
import type { Issue, Release, Repository, Worklane } from "@multica/core/types";
import { typeFilter, type IssueTypePick } from "@multica/core/worklanes";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import { ReleaseIssueRow } from "../../releases/components/release-issue-row";
import { PlanReleaseButton, ReleaseLine } from "../../releases/components/release-line";

const EMPTY: never[] = [];

/**
 * The worklane's Timeline: its repository's release line followed by Plan
 * release, or "No releases yet" and Plan release while the repository has
 * none. A repository that does not release says so in place of both. Each
 * release column lists the worklane's issues in it of the picked types, and
 * an issue opens its preview.
 */
export function WorklaneTimeline({
  worklane,
  repository,
  issueTypes,
}: {
  worklane: Worklane;
  repository: Repository;
  issueTypes: readonly IssueTypePick[];
}) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const { data: releases = EMPTY, isLoading } = useQuery(releaseListOptions(wsId));
  const line = selectLine(releases, repository.id);
  const { data: issues = EMPTY } = useQuery(worklaneIssuesOptions(wsId, worklane.id));
  const byRelease = useMemo(() => {
    const map = new Map<string, Issue[]>();
    for (const issue of issues.filter(typeFilter(issueTypes))) {
      if (!issue.release_id) continue;
      const list = map.get(issue.release_id);
      if (list) list.push(issue);
      else map.set(issue.release_id, [issue]);
    }
    return map;
  }, [issues, issueTypes]);
  const renderIssues = (release: Release) => {
    const inRelease = byRelease.get(release.id);
    if (!inRelease) return null;
    return (
      <div className="flex flex-col py-1">
        {inRelease.map((issue) => (
          <ReleaseIssueRow key={issue.id} issue={issue} />
        ))}
      </div>
    );
  };

  return (
    <section aria-labelledby="worklane-timeline-title" className="space-y-3">
      <h2 id="worklane-timeline-title" className="text-body font-medium text-foreground">
        {t(($) => $.timeline.title)}
      </h2>
      {isLoading ? (
        <Skeleton className="h-16 w-full" />
      ) : line.length > 0 ? (
        <ReleaseLine repository={repository} worklane={worklane} renderIssues={renderIssues} />
      ) : repository.version_policy === "none" ? (
        <PlanReleaseButton repository={repository} worklane={worklane} />
      ) : (
        <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed px-4 py-6">
          <p className="text-center text-body text-muted-foreground">
            {t(($) => $.timeline.no_releases)}
          </p>
          <PlanReleaseButton repository={repository} worklane={worklane} />
        </div>
      )}
    </section>
  );
}
