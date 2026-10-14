"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FolderGit2, GitBranch } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { worklaneIssuesOptions } from "@multica/core/issues/placement";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { lastShipped, formatVersion } from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import type { Repository } from "@multica/core/types";
import { useWorklane, type IssueTypePick } from "@multica/core/worklanes";
import { Tabs, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { BreadcrumbHeader } from "../../layout/breadcrumb-header";
import { CollectionPageState } from "../../layout/collection-page";
import { PAGE_GUTTER } from "../../layout/page-header";
import { useT } from "../../i18n";
import { useReleaseLine } from "../../releases/use-release-line";
import { IssuePeekHost } from "../../issues/components/issue-peek";
import { WorklaneActivity } from "./worklane-activity";
import { WorklaneBoard } from "./worklane-board";
import { WorklaneList } from "./worklane-list";
import { WorklaneMenu } from "./worklane-menu";
import { WorklaneHead } from "./worklane-state";
import { WorklaneTimeline } from "./worklane-timeline";
import { WorklaneTypeFilter } from "./worklane-type-filter";

type WorklaneView = "timeline" | "board" | "list";

const VIEWS: readonly WorklaneView[] = ["timeline", "board", "list"];

const NO_ISSUES: never[] = [];

/**
 * A worklane's page: the breadcrumb Projects › {project} › {owner/name} ›
 * {worklane}, the repository, Busy or Free with now or next, then the view
 * switch and the Type filter over Timeline (with the activity), Board and
 * List. The page opens on Timeline; the view and the picked types are local
 * state, so switching views keeps the types. Every issue opens its preview
 * beside the view. Every worklane but `main` carries its menu in the header.
 */
export function WorklanePage({ worklaneId }: { worklaneId: string }) {
  const { t } = useT("worklanes");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const [view, setView] = useState<WorklaneView>("timeline");
  const [issueTypes, setIssueTypes] = useState<IssueTypePick[]>([]);
  const { data: worklane, isLoading: worklaneLoading } = useWorklane(wsId, worklaneId);
  const { data: issues = NO_ISSUES } = useQuery(worklaneIssuesOptions(wsId, worklaneId));
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
    <IssuePeekHost openMode="peek">
      <div className="flex flex-1 min-h-0 flex-col">
        <BreadcrumbHeader
          segments={breadcrumb}
          leaf={<span className="truncate font-medium text-foreground">{worklane.name}</span>}
          actions={<WorklaneMenu worklane={worklane} />}
        />
        <div className={cn("flex flex-col gap-4 pt-6 pb-4", PAGE_GUTTER)}>
          <header className="flex items-start gap-4">
            <span className="flex size-12 shrink-0 items-center justify-center rounded-lg border bg-muted/40 text-muted-foreground">
              <GitBranch aria-hidden="true" className="size-5" />
            </span>
            <div className="min-w-0 space-y-1.5">
              <h1 className="truncate text-xl font-semibold text-foreground">{worklane.name}</h1>
              <div className="flex min-w-0 flex-wrap items-center gap-3">
                <span
                  aria-label={t(($) => $.page.repository)}
                  className="inline-flex items-center gap-1.5 rounded-md border px-1.5 py-0.5 font-mono text-caption text-foreground"
                >
                  <FolderGit2 aria-hidden="true" className="size-3.5 text-muted-foreground" />
                  {repository.full_name}
                </span>
                <WorklaneHead worklane={worklane} />
                <WorklaneReleaseStatus repository={repository} />
              </div>
            </div>
          </header>
          <div className="flex flex-wrap items-center gap-3">
            <Tabs value={view} onValueChange={(next) => setView(next as WorklaneView)}>
              <TabsList aria-label={t(($) => $.views.label)}>
                {VIEWS.map((option) => (
                  <TabsTrigger key={option} value={option}>
                    {t(($) => $.views[option])}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            <WorklaneTypeFilter issues={issues} value={issueTypes} onChange={setIssueTypes} />
          </div>
        </div>
        {view === "board" ? (
          <div className="flex flex-1 min-h-0 flex-col">
            <WorklaneBoard worklaneId={worklane.id} issueTypes={issueTypes} />
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto">
            <div className={cn("flex flex-col gap-8 pb-6", PAGE_GUTTER)}>
              {view === "timeline" ? (
                <>
                  <WorklaneTimeline
                    worklane={worklane}
                    repository={repository}
                    issueTypes={issueTypes}
                  />
                  <WorklaneActivity worklane={worklane} />
                </>
              ) : (
                <WorklaneList worklane={worklane} repository={repository} issueTypes={issueTypes} />
              )}
            </div>
          </div>
        )}
      </div>
    </IssuePeekHost>
  );
}

/**
 * The header's release status: "does not release", "last shipped
 * v{version}" or "nothing shipped yet" while the repository has shipped no
 * release. A release shows no date or goal until it ships.
 */
function WorklaneReleaseStatus({ repository }: { repository: Repository }) {
  const { t } = useT("worklanes");
  const { line, isLoading } = useReleaseLine(repository.id, repository.last_tag);
  if (isLoading) return null;
  const shipped = lastShipped(line);
  return (
    <span className="text-caption text-muted-foreground">
      {repository.version_policy === "none"
        ? t(($) => $.page.does_not_release)
        : shipped
          ? t(($) => $.page.last_shipped, { version: formatVersion(shipped.version) })
          : t(($) => $.page.nothing_shipped)}
    </span>
  );
}
