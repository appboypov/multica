"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useModalStore } from "@multica/core/modals";
import { releaseProgressOptions, worklaneIssuesOptions } from "@multica/core/issues/placement";
import { releaseListOptions, selectLine } from "@multica/core/releases";
import type { Issue, Release, Repository, Worklane } from "@multica/core/types";
import { typeFilter, type IssueTypePick } from "@multica/core/worklanes";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import { NoReleaseColumn } from "../../releases/components/no-release-column";
import { ReleaseIssueRow } from "../../releases/components/release-issue-row";
import { PlanReleaseButton, ReleaseLine } from "../../releases/components/release-line";
import { ReleaseMilestones } from "../../releases/components/release-milestones";

const EMPTY: never[] = [];

/**
 * The worklane's Timeline: its repository's release line followed by "No
 * release" and Plan release, or "No releases yet" and Plan release while the
 * repository has none. A repository that does not release says so in place
 * of both, and still shows "No release". Each release column lists the
 * worklane's issues in it of the picked types with their progress, and an
 * unshipped release offers "+ Add issue to release", which opens New issue in
 * it. An issue opens its preview.
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
  const { t: tReleases } = useT("releases");
  const wsId = useWorkspaceId();
  const openModal = useModalStore((s) => s.open);
  const { data: releases = EMPTY, isLoading } = useQuery(releaseListOptions(wsId));
  const line = selectLine(releases, repository.id);
  const { data: issues = EMPTY } = useQuery(worklaneIssuesOptions(wsId, worklane.id));
  const { data: progress } = useQuery(releaseProgressOptions(wsId));
  const { byRelease, noRelease } = useMemo(() => {
    const map = new Map<string, Issue[]>();
    const none: Issue[] = [];
    for (const issue of issues.filter(typeFilter(issueTypes))) {
      if (!issue.release_id) {
        none.push(issue);
        continue;
      }
      const list = map.get(issue.release_id);
      if (list) list.push(issue);
      else map.set(issue.release_id, [issue]);
    }
    return { byRelease: map, noRelease: none };
  }, [issues, issueTypes]);

  const renderIssues = (release: Release) => {
    const inRelease = byRelease.get(release.id);
    const unshipped = release.shipped_at === null;
    const row = progress?.rows.find(
      (candidate) => candidate.release_id === release.id && candidate.worklane_id === worklane.id,
    );
    return (
      <div className="flex flex-col py-1">
        <div className="px-4 py-2"><ReleaseMilestones release={release} /></div>
        {row && row.total > 0 ? (
          <p className="px-4 pb-1 text-caption tabular-nums text-muted-foreground">
            {tReleases(($) => $.issues.progress, { done: row.done, total: row.total })}
          </p>
        ) : null}
        {inRelease ? (
          inRelease.map((issue) => <ReleaseIssueRow key={issue.id} issue={issue} release={release} />)
        ) : (
          <p className="px-4 py-2 text-label text-muted-foreground">{tReleases(($) => $.column.no_issues)}</p>
        )}
        {unshipped ? (
          <button
            type="button"
            onClick={() => openModal("create-issue", { worklane_id: worklane.id, release_id: release.id })}
            className="mx-2 mt-1 inline-flex items-center gap-1.5 rounded-md px-2 py-1.5 text-caption text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            <Plus className="size-3.5" aria-hidden="true" />
            {tReleases(($) => $.issues.add_issue)}
          </button>
        ) : null}
      </div>
    );
  };
  const noReleaseColumn = <NoReleaseColumn issues={noRelease} />;

  return (
    <section aria-labelledby="worklane-timeline-title" className="space-y-3">
      <h2 id="worklane-timeline-title" className="text-body font-medium text-foreground">
        {t(($) => $.timeline.title)}
      </h2>
      {isLoading ? (
        <Skeleton className="h-16 w-full" />
      ) : line.length > 0 ? (
        <ReleaseLine
          repository={repository}
          worklane={worklane}
          renderIssues={renderIssues}
          trailing={noReleaseColumn}
        />
      ) : repository.version_policy === "none" ? (
        <div className="space-y-3">
          <PlanReleaseButton repository={repository} worklane={worklane} />
          <div className="overflow-x-auto pb-2">{noReleaseColumn}</div>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed px-4 py-6">
            <p className="text-center text-body text-muted-foreground">
              {t(($) => $.timeline.no_releases)}
            </p>
            <PlanReleaseButton repository={repository} worklane={worklane} />
          </div>
          <div className="overflow-x-auto pb-2">{noReleaseColumn}</div>
        </div>
      )}
    </section>
  );
}
