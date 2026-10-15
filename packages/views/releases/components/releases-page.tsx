"use client";

import { useMemo, useState } from "react";
import { Tag } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { releaseProgressOptions } from "@multica/core/issues/placement";
import { useOrganisations } from "@multica/core/organisations";
import { projectListOptions, useProjectViewStore } from "@multica/core/projects";
import { releaseListOptions, releaseOverview } from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import { worklaneListOptions } from "@multica/core/worklanes";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { CollectionPageHeader, CollectionPageState } from "../../layout/collection-page";
import { WORK_TREE_INDENT, WorkTree } from "../../projects/components/work-tree";
import { useT } from "../../i18n";
import { RELEASE_ROW_COLS, ReleaseRow } from "./release-row";

const EMPTY_PROGRESS = { rows: [], releases: [] };

/**
 * Releases: every repository's releases in line order under their
 * project and organisation, built in the client from the workspace's lists
 * (ADR-0016). Every group opens unfolded; the fold state is not kept. It only
 * shows releases: planning, editing and moving them stay on a worklane's
 * Timeline.
 */
export function ReleasesPage() {
  const { t } = useT("releases");
  const wsId = useWorkspaceId();
  const sortField = useProjectViewStore((s) => s.sortField);
  const sortDirection = useProjectViewStore((s) => s.sortDirection);
  const organisations = useOrganisations(wsId);
  const projects = useQuery(projectListOptions(wsId));
  const repositories = useQuery(repositoryListOptions(wsId));
  const worklanes = useQuery(worklaneListOptions(wsId));
  const releases = useQuery(releaseListOptions(wsId));
  const progress = useQuery(releaseProgressOptions(wsId));
  const [closed, setClosed] = useState<ReadonlySet<string>>(() => new Set());

  const tree = useMemo(
    () =>
      releaseOverview({
        organisations: organisations.data ?? [],
        projects: projects.data ?? [],
        repositories: repositories.data ?? [],
        worklanes: worklanes.data ?? [],
        releases: releases.data ?? [],
        progress: progress.data ?? EMPTY_PROGRESS,
        order: { field: sortField, direction: sortDirection },
      }),
    [
      organisations.data,
      projects.data,
      repositories.data,
      worklanes.data,
      releases.data,
      progress.data,
      sortField,
      sortDirection,
    ],
  );

  const toggle = (key: string) =>
    setClosed((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const isLoading =
    organisations.isLoading ||
    projects.isLoading ||
    repositories.isLoading ||
    worklanes.isLoading ||
    releases.isLoading ||
    progress.isLoading;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionPageHeader icon={Tag} title={t(($) => $.overview.title)} count={tree.total} />
      {isLoading ? (
        <div className="space-y-2 p-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </div>
      ) : tree.groups.length === 0 ? (
        <CollectionPageState icon={Tag} title={t(($) => $.overview.empty)} />
      ) : (
        <div className="min-h-0 flex-1 overflow-auto pb-16">
          <WorkTree
            tree={tree}
            gridClassName={RELEASE_ROW_COLS}
            columns={[
              t(($) => $.overview.column_name),
              t(($) => $.overview.column_issues),
              t(($) => $.overview.column_worklanes),
            ]}
            organisationMeta={(count) => t(($) => $.overview.release_count, { count })}
            projectMeta={(repositoryCount, count) =>
              t(($) => $.overview.project_meta, {
                repositories: t(($) => $.overview.repository_count, { count: repositoryCount }),
                releases: t(($) => $.overview.release_count, { count }),
              })
            }
            renderRepository={(entry) =>
              entry.doesNotRelease ? (
                <div
                  className={cn(
                    "col-span-full flex h-10 items-center border-b pr-3 text-caption text-muted-foreground",
                    WORK_TREE_INDENT.rows,
                  )}
                >
                  {t(($) => $.overview.does_not_release)}
                </div>
              ) : (
                entry.releases.map((row) => (
                  <ReleaseRow key={row.release.id} row={row} indent={WORK_TREE_INDENT.rows} />
                ))
              )
            }
            closed={closed}
            onToggle={toggle}
          />
        </div>
      )}
    </div>
  );
}
