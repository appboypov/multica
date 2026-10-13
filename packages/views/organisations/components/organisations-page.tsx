"use client";

import { useMemo } from "react";
import { Building2, ChevronRight, Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useModalStore } from "@multica/core/modals";
import { useOrganisations } from "@multica/core/organisations";
import { projectListOptions } from "@multica/core/projects";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import {
  ListGrid,
  ListGridCell,
  ListGridHeader,
  ListGridHeaderCell,
  ListGridRow,
  LIST_GRID_BOTTOM_CLEARANCE,
} from "@multica/ui/components/ui/list-grid";
import {
  CollectionPageHeader,
  CollectionPageHeaderAction,
  CollectionPageState,
} from "../../layout/collection-page";
import { useRowLink } from "../../navigation";
import { useT } from "../../i18n";

// Edge padding, name, project count, chevron, edge padding.
const GRID_COLS = "grid-cols-[0.75rem_minmax(160px,1fr)_8rem_1.75rem_0.75rem]";

const openCreateOrganisation = () => useModalStore.getState().open("create-organisation");

/**
 * Organisations: every organisation of the workspace by name, each with the
 * number of projects that point to it. A row opens the organisation's page.
 */
export function OrganisationsPage() {
  const { t } = useT("organisations");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const rowLink = useRowLink();
  const { data: organisations = [], isLoading } = useOrganisations(wsId);
  const { data: projects = [] } = useQuery(projectListOptions(wsId));

  const projectCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const project of projects) {
      if (!project.organisation_id) continue;
      counts.set(project.organisation_id, (counts.get(project.organisation_id) ?? 0) + 1);
    }
    return counts;
  }, [projects]);

  const rows = useMemo(
    () =>
      [...organisations].sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: "base" }),
      ),
    [organisations],
  );

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <CollectionPageHeader
        icon={Building2}
        title={t(($) => $.page.title)}
        count={organisations.length}
        actions={
          <CollectionPageHeaderAction
            icon={Plus}
            label={t(($) => $.page.new_organisation)}
            onClick={openCreateOrganisation}
          />
        }
      />

      {isLoading ? (
        <LoadingSkeleton />
      ) : rows.length === 0 ? (
        <CollectionPageState
          icon={Building2}
          title={t(($) => $.page.empty)}
          actions={
            <Button size="sm" variant="outline" onClick={openCreateOrganisation}>
              <Plus aria-hidden="true" className="size-3.5" />
              {t(($) => $.page.new_organisation)}
            </Button>
          }
        />
      ) : (
        <div className="min-h-0 flex-1 overflow-auto @container">
          <ListGrid className={GRID_COLS} style={{ paddingBottom: LIST_GRID_BOTTOM_CLEARANCE }}>
            <ListGridHeader>
              <ListGridHeaderCell>{t(($) => $.table.name)}</ListGridHeaderCell>
              <ListGridHeaderCell>{t(($) => $.table.projects)}</ListGridHeaderCell>
              <span aria-hidden="true" />
            </ListGridHeader>
            {rows.map((organisation) => (
              <ListGridRow
                key={organisation.id}
                className="cursor-pointer"
                {...rowLink(paths.organisationDetail(organisation.id), organisation.name)}
              >
                <ListGridCell className="gap-3">
                  <span className="flex size-6 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                    <Building2 aria-hidden="true" className="size-3.5" />
                  </span>
                  <span className="min-w-0 truncate text-body font-medium">
                    {organisation.name}
                  </span>
                </ListGridCell>
                <ListGridCell className="text-caption tabular-nums text-muted-foreground">
                  {t(($) => $.project_count, { count: projectCounts.get(organisation.id) ?? 0 })}
                </ListGridCell>
                <ListGridCell className="justify-end px-0 text-muted-foreground">
                  <ChevronRight aria-hidden="true" className="size-4" />
                </ListGridCell>
              </ListGridRow>
            ))}
          </ListGrid>
        </div>
      )}
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="min-h-0 flex-1 overflow-auto @container">
      <ListGrid className={GRID_COLS}>
        {Array.from({ length: 6 }).map((_, i) => (
          <ListGridRow key={i}>
            <ListGridCell className="gap-3">
              <Skeleton className="size-6 rounded-md" />
              <Skeleton className="h-4 w-40" />
            </ListGridCell>
            <ListGridCell>
              <Skeleton className="h-3 w-16" />
            </ListGridCell>
            <ListGridCell className="px-0" />
          </ListGridRow>
        ))}
      </ListGrid>
    </div>
  );
}
