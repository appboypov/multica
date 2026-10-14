"use client";

import { useMemo, useState } from "react";
import { GitBranch, Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useModalStore } from "@multica/core/modals";
import { useOrganisations } from "@multica/core/organisations";
import { projectListOptions, useProjectViewStore } from "@multica/core/projects";
import { repositoryListOptions } from "@multica/core/repositories";
import { worklaneListOptions, worklaneOverview } from "@multica/core/worklanes";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { CollectionPageHeader, CollectionPageState } from "../../layout/collection-page";
import { WORK_TREE_INDENT, WorkTree } from "../../projects/components/work-tree";
import { useT } from "../../i18n";
import { WORKLANE_TREE_COLS, WorklaneTreeRow } from "./worklane-tree-row";

/**
 * Worklanes: every worklane of the workspace under its repository, project
 * and organisation, built in the client from the workspace's lists
 * (ADR-0016). Every group opens unfolded; the fold state is not kept.
 */
export function WorklanesPage() {
  const { t } = useT("worklanes");
  const { t: tReleases } = useT("releases");
  const wsId = useWorkspaceId();
  const sortField = useProjectViewStore((s) => s.sortField);
  const sortDirection = useProjectViewStore((s) => s.sortDirection);
  const organisations = useOrganisations(wsId);
  const projects = useQuery(projectListOptions(wsId));
  const repositories = useQuery(repositoryListOptions(wsId));
  const worklanes = useQuery(worklaneListOptions(wsId));
  const [closed, setClosed] = useState<ReadonlySet<string>>(() => new Set());

  const tree = useMemo(
    () =>
      worklaneOverview({
        organisations: organisations.data ?? [],
        projects: projects.data ?? [],
        repositories: repositories.data ?? [],
        worklanes: worklanes.data ?? [],
        order: { field: sortField, direction: sortDirection },
      }),
    [organisations.data, projects.data, repositories.data, worklanes.data, sortField, sortDirection],
  );

  const toggle = (key: string) =>
    setClosed((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const isLoading =
    organisations.isLoading || projects.isLoading || repositories.isLoading || worklanes.isLoading;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionPageHeader icon={GitBranch} title={t(($) => $.overview.title)} count={tree.total} />
      {isLoading ? (
        <div className="space-y-2 p-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </div>
      ) : tree.total === 0 ? (
        <CollectionPageState icon={GitBranch} title={t(($) => $.overview.empty)} />
      ) : (
        <div className="min-h-0 flex-1 overflow-auto pb-16">
          <WorkTree
            tree={tree}
            gridClassName={WORKLANE_TREE_COLS}
            columns={[t(($) => $.overview.column_name), tReleases(($) => $.chips.column)]}
            organisationMeta={(count) => t(($) => $.worklane_count, { count })}
            projectMeta={(repositoryCount, count) =>
              t(($) => $.overview.project_meta, {
                repositories: t(($) => $.overview.repository_count, { count: repositoryCount }),
                worklanes: t(($) => $.worklane_count, { count }),
              })
            }
            repositoryAction={(repository) => (
              <Button
                size="xs"
                variant="ghost"
                aria-label={t(($) => $.rows.add_label, { repository: repository.full_name })}
                onClick={() =>
                  useModalStore.getState().open("new-worklane", { repositoryId: repository.id })
                }
              >
                <Plus aria-hidden="true" />
                {t(($) => $.rows.add)}
              </Button>
            )}
            renderRepository={(entry) =>
              entry.worklanes.map((worklane) => (
                <WorklaneTreeRow key={worklane.id} worklane={worklane} indent={WORK_TREE_INDENT.rows} />
              ))
            }
            closed={closed}
            onToggle={toggle}
          />
        </div>
      )}
    </div>
  );
}
