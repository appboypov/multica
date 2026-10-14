"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { releaseListOptions, selectLine } from "@multica/core/releases";
import type { Repository, Worklane } from "@multica/core/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import { PlanReleaseButton, ReleaseLine } from "../../releases/components/release-line";

const EMPTY: never[] = [];

/**
 * The worklane's Timeline: its repository's release line followed by Plan
 * release, or "No releases yet" and Plan release while the repository has
 * none. A repository that does not release says so in place of both.
 */
export function WorklaneTimeline({
  worklane,
  repository,
}: {
  worklane: Worklane;
  repository: Repository;
}) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const { data: releases = EMPTY, isLoading } = useQuery(releaseListOptions(wsId));
  const line = selectLine(releases, repository.id);

  return (
    <section aria-labelledby="worklane-timeline-title" className="space-y-3">
      <h2 id="worklane-timeline-title" className="text-body font-medium text-foreground">
        {t(($) => $.timeline.title)}
      </h2>
      {isLoading ? (
        <Skeleton className="h-16 w-full" />
      ) : line.length > 0 ? (
        <ReleaseLine repository={repository} worklane={worklane} />
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
