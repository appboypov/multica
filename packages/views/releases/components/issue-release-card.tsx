"use client";

import { Milestone } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { releaseProgressOptions } from "@multica/core/issues/placement";
import { formatVersion } from "@multica/core/releases";
import type { Issue } from "@multica/core/types";
import { AppLink } from "../../navigation";
import { usePlacementData } from "../../issues/components/pickers";
import { useT } from "../../i18n";

/**
 * The issue page's release card: the issue's release with its version, title
 * and description, its repository and worklane, the progress of the
 * worklane's issues in it, and a link to the worklane's Timeline. An issue in
 * No release shows no card.
 */
export function IssueReleaseCard({ issue }: { issue: Issue }) {
  const { t } = useT("releases");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const data = usePlacementData();
  const { data: progress } = useQuery(releaseProgressOptions(wsId));

  const release = issue.release_id ? data.releases.find((r) => r.id === issue.release_id) : undefined;
  if (!release) return null;
  const worklane = issue.worklane_id ? data.worklanes.find((w) => w.id === issue.worklane_id) : undefined;
  const repository = data.repositories.find((r) => r.id === release.repository_id);
  const row = progress?.rows.find(
    (candidate) => candidate.release_id === release.id && candidate.worklane_id === issue.worklane_id,
  );
  const totals = row ?? progress?.releases.find((candidate) => candidate.release_id === release.id);
  const version = formatVersion(release.version);

  return (
    <section
      aria-label={t(($) => $.issue_card.aria, { version, title: release.title })}
      className="rounded-lg border bg-card p-3 text-body"
    >
      <div className="flex items-center gap-2">
        <Milestone aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        <span className="font-medium tabular-nums">{version}</span>
        <span className="min-w-0 truncate">{release.title}</span>
      </div>
      {release.description && (
        <p className="mt-1.5 line-clamp-3 text-caption text-muted-foreground">{release.description}</p>
      )}
      <p className="mt-1.5 truncate text-caption text-muted-foreground">
        {[repository?.full_name, worklane?.name].filter(Boolean).join(" · ")}
      </p>
      <div className="mt-2 flex items-center justify-between gap-2">
        {totals && totals.total > 0 ? (
          <span className="text-caption tabular-nums text-muted-foreground">
            {t(($) => $.issues.progress, { done: totals.done, total: totals.total })}
          </span>
        ) : (
          <span />
        )}
        {worklane && (
          <AppLink
            href={`${paths.worklaneDetail(worklane.id)}?release=${encodeURIComponent(release.id)}`}
            className="text-caption text-primary hover:underline"
          >
            {t(($) => $.issue_card.open_timeline)}
          </AppLink>
        )}
      </div>
    </section>
  );
}
