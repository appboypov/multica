"use client";

import type { MouseEvent, ReactNode } from "react";
import { Building2, ChevronRight, GitBranch } from "lucide-react";
import type { WorkTree as WorkTreeData } from "@multica/core/projects";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Repository } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../../navigation";
import { ActorAvatar } from "../../common/actor-avatar";
import { useT } from "../../i18n";
import { ProjectIcon } from "./project-icon";

/** The fold keys of the tree's rows: `org:{id|none}`, `project:{id}`, `repository:{id}`. */
export const workTreeKey = {
  organisation: (id: string | null) => `org:${id ?? "none"}`,
  project: (id: string) => `project:${id}`,
  repository: (id: string) => `repository:${id}`,
};

/**
 * Indents of the tree's levels. Literal classes so Tailwind sees them; a
 * page's rows under a repository take `rows`.
 */
export const WORK_TREE_INDENT = {
  organisation: "pl-3",
  project: "pl-8",
  repository: "pl-13",
  rows: "pl-[4.75rem]",
} as const;

function FoldButton({
  name,
  open,
  onToggle,
}: {
  name: string;
  open: boolean;
  onToggle: () => void;
}) {
  const { t } = useT("projects");
  return (
    <button
      type="button"
      aria-expanded={open}
      aria-label={t(($) => $.group.toggle_aria, { name })}
      onClick={onToggle}
      className="-ml-1 flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
    >
      <ChevronRight
        aria-hidden="true"
        className={cn("size-3.5 transition-transform motion-reduce:transition-none", open && "rotate-90")}
      />
    </button>
  );
}

// A group row folds from anywhere outside its links and buttons, which handle
// their own clicks.
function GroupRow({
  indent,
  onToggle,
  children,
}: {
  indent: string;
  onToggle: () => void;
  children: ReactNode;
}) {
  const onClick = (event: MouseEvent<HTMLDivElement>) => {
    if ((event.target as Element).closest("a, button")) return;
    onToggle();
  };
  return (
    <div
      role="row"
      onClick={onClick}
      className={cn(
        "col-span-full flex h-10 min-w-0 cursor-pointer items-center gap-1.5 border-b pr-3 transition-colors hover:bg-accent/40",
        indent,
      )}
    >
      {children}
    </div>
  );
}

/**
 * The organisation, project and repository tree of Worklanes and Releases
 * (ADR-0016). Every group starts open; the page holds the closed keys and
 * does not persist them. Under each open repository the page renders its
 * own rows through `renderRepository`.
 */
export function WorkTree<R extends { repository: Repository }>({
  tree,
  gridClassName,
  columns,
  organisationMeta,
  projectMeta,
  repositoryAction,
  renderRepository,
  closed,
  onToggle,
}: {
  tree: WorkTreeData<R>;
  /** The literal `grid-cols-[...]` template the header and the page's rows share. */
  gridClassName: string;
  columns: ReactNode[];
  organisationMeta: (count: number) => string;
  projectMeta: (repositoryCount: number, count: number) => string;
  repositoryAction?: (repository: Repository) => ReactNode;
  renderRepository: (entry: R) => ReactNode;
  closed: ReadonlySet<string>;
  onToggle: (key: string) => void;
}) {
  const { t } = useT("projects");
  const wsPaths = useWorkspacePaths();

  return (
    <div role="table" className="grid min-w-0 grid-cols-1">
      <div
        role="row"
        className={cn(
          "sticky top-0 z-10 grid h-9 items-center gap-x-3 border-b bg-background pr-3 text-caption font-medium text-muted-foreground",
          WORK_TREE_INDENT.organisation,
          gridClassName,
        )}
      >
        {columns.map((column, index) => (
          <span key={index} role="columnheader" className="min-w-0 truncate">
            {column}
          </span>
        ))}
      </div>
      {tree.groups.map((group) => {
        const { organisation } = group;
        const orgKey = workTreeKey.organisation(organisation?.id ?? null);
        const orgOpen = !closed.has(orgKey);
        const orgName = organisation?.name ?? t(($) => $.organisation.none);
        return (
          <div key={orgKey} role="rowgroup" className="contents">
            <GroupRow indent={WORK_TREE_INDENT.organisation} onToggle={() => onToggle(orgKey)}>
              <FoldButton name={orgName} open={orgOpen} onToggle={() => onToggle(orgKey)} />
              <Building2 aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
              {organisation ? (
                <AppLink
                  href={wsPaths.organisationDetail(organisation.id)}
                  className="min-w-0 truncate text-caption font-medium hover:underline"
                >
                  {orgName}
                </AppLink>
              ) : (
                <span className="min-w-0 truncate text-caption font-medium">{orgName}</span>
              )}
              <span className="shrink-0 text-caption text-muted-foreground">
                {organisationMeta(group.count)}
              </span>
            </GroupRow>
            {orgOpen &&
              group.projects.map((entry) => {
                const { project } = entry;
                const projectKey = workTreeKey.project(project.id);
                const projectOpen = !closed.has(projectKey);
                return (
                  <div key={projectKey} role="rowgroup" className="contents">
                    <GroupRow indent={WORK_TREE_INDENT.project} onToggle={() => onToggle(projectKey)}>
                      <FoldButton
                        name={project.title}
                        open={projectOpen}
                        onToggle={() => onToggle(projectKey)}
                      />
                      <ProjectIcon project={project} size="sm" />
                      <AppLink
                        href={wsPaths.projectDetail(project.id)}
                        className="min-w-0 truncate text-caption font-medium hover:underline"
                      >
                        {project.title}
                      </AppLink>
                      <span className="shrink-0 text-caption text-muted-foreground">
                        {projectMeta(entry.repositoryCount, entry.count)}
                      </span>
                      {project.lead_type && project.lead_id && (
                        <span className="ml-auto flex shrink-0 items-center">
                          <ActorAvatar
                            actorType={project.lead_type}
                            actorId={project.lead_id}
                            size="sm"
                            enableHoverCard
                          />
                        </span>
                      )}
                    </GroupRow>
                    {projectOpen &&
                      entry.repositories.map((repositoryEntry) => {
                        const { repository } = repositoryEntry;
                        const repositoryKey = workTreeKey.repository(repository.id);
                        const repositoryOpen = !closed.has(repositoryKey);
                        return (
                          <div key={repositoryKey} role="rowgroup" className="contents">
                            <GroupRow
                              indent={WORK_TREE_INDENT.repository}
                              onToggle={() => onToggle(repositoryKey)}
                            >
                              <FoldButton
                                name={repository.full_name}
                                open={repositoryOpen}
                                onToggle={() => onToggle(repositoryKey)}
                              />
                              <span className="min-w-0 truncate text-caption font-medium">
                                {repository.full_name}
                              </span>
                              <span className="flex shrink-0 items-center gap-1 text-caption text-muted-foreground">
                                <GitBranch aria-hidden="true" className="size-3" />
                                {repository.default_branch}
                              </span>
                              {repositoryAction && (
                                <span className="ml-auto flex shrink-0 items-center">
                                  {repositoryAction(repository)}
                                </span>
                              )}
                            </GroupRow>
                            {repositoryOpen && renderRepository(repositoryEntry)}
                          </div>
                        );
                      })}
                  </div>
                );
              })}
          </div>
        );
      })}
    </div>
  );
}
