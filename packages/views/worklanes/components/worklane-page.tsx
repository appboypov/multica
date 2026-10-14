"use client";

import { useQuery } from "@tanstack/react-query";
import { FolderGit2, GitBranch } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { repositoryListOptions } from "@multica/core/repositories";
import { useWorklane } from "@multica/core/worklanes";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { BreadcrumbHeader } from "../../layout/breadcrumb-header";
import { CollectionPageState } from "../../layout/collection-page";
import { PAGE_GUTTER } from "../../layout/page-header";
import { useT } from "../../i18n";
import { WorklaneActivity } from "./worklane-activity";
import { WorklaneMenu } from "./worklane-menu";
import { WorklaneStateLabel } from "./worklane-state";
import { WorklaneTimeline } from "./worklane-timeline";

/**
 * A worklane's page: the breadcrumb Projects › {project} › {owner/name} ›
 * {worklane}, the repository and the state, then its Timeline and its
 * activity. Every worklane but `main` carries its menu in the header.
 */
export function WorklanePage({ worklaneId }: { worklaneId: string }) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { data: worklane, isLoading: worklaneLoading } = useWorklane(wsId, worklaneId);
  const { data: repositories, isLoading: repositoriesLoading } = useQuery(
    repositoryListOptions(wsId),
  );
  const repository = worklane
    ? repositories?.find((candidate) => candidate.id === worklane.repository_id)
    : undefined;
  const { data: project } = useQuery({
    ...projectDetailOptions(wsId, repository?.project_id ?? ""),
    enabled: !!repository,
  });

  const projectsSegment = { href: paths.projects(), label: t(($) => $.page.projects) };

  if (worklaneLoading || repositoriesLoading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <BreadcrumbHeader segments={[projectsSegment]} leaf={<Skeleton className="h-4 w-32" />} />
        <div aria-label={t(($) => $.page.loading)} className={cn("space-y-3 py-6", PAGE_GUTTER)}>
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-64" />
        </div>
      </div>
    );
  }

  if (!worklane || !repository) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <BreadcrumbHeader
          segments={[projectsSegment]}
          leaf={<span className="truncate text-muted-foreground">{t(($) => $.page.not_found)}</span>}
        />
        <CollectionPageState
          icon={GitBranch}
          title={t(($) => $.page.not_found)}
          description={t(($) => $.page.not_found_description)}
        />
      </div>
    );
  }

  const projectHref = paths.projectDetail(repository.project_id);
  const breadcrumb = [
    projectsSegment,
    {
      href: projectHref,
      label: project ? project.title : <Skeleton className="h-4 w-20" />,
    },
    { href: projectHref, label: repository.full_name },
  ];

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <BreadcrumbHeader
        segments={breadcrumb}
        leaf={<span className="truncate font-medium text-foreground">{worklane.name}</span>}
        actions={<WorklaneMenu worklane={worklane} />}
      />
      <div className="flex-1 overflow-y-auto">
        <div className={cn("flex flex-col gap-8 py-6", PAGE_GUTTER)}>
          <header className="flex items-start gap-4">
            <span className="flex size-12 shrink-0 items-center justify-center rounded-lg border bg-muted/40 text-muted-foreground">
              <GitBranch aria-hidden="true" className="size-5" />
            </span>
            <div className="min-w-0 space-y-1.5">
              <h1 className="truncate text-xl font-semibold text-foreground">{worklane.name}</h1>
              <div className="flex flex-wrap items-center gap-3">
                <span
                  aria-label={t(($) => $.page.repository)}
                  className="inline-flex items-center gap-1.5 rounded-md border px-1.5 py-0.5 font-mono text-caption text-foreground"
                >
                  <FolderGit2 aria-hidden="true" className="size-3.5 text-muted-foreground" />
                  {repository.full_name}
                </span>
                <WorklaneStateLabel state={worklane.state} />
              </div>
            </div>
          </header>
          <WorklaneTimeline worklane={worklane} repository={repository} />
          <WorklaneActivity worklane={worklane} />
        </div>
      </div>
    </div>
  );
}
