"use client";

import { useCallback, useMemo, useState } from "react";
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  Filter,
  FolderKanban,
  LayoutGrid,
  Plus,
  Rows3,
  Search,
  X,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import {
  projectListOptions,
  useUpdateProject,
  useProjectViewStore,
  type ProjectColumnKey,
  type ProjectListFilters,
  type ProjectSortField,
  type ProjectViewMode,
} from "@multica/core/projects";
import { pinListOptions } from "@multica/core/pins";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useAuthStore } from "@multica/core/auth";
import { useActorName } from "@multica/core/workspace/hooks";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useModalStore } from "@multica/core/modals";
import { AppLink } from "../../navigation";
import { ActorAvatar } from "../../common/actor-avatar";
import { FILTER_ITEM_CLASS, HoverCheck } from "../../common/hover-check";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { LIST_GRID_BOTTOM_CLEARANCE } from "@multica/ui/components/ui/list-grid";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import { Switch } from "@multica/ui/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import type {
  MemberWithUser,
  Project,
  ProjectPriority,
  ProjectStatus,
  UpdateProjectRequest,
} from "@multica/core/types";
import {
  CollectionPageHeader,
  CollectionPageHeaderAction,
  CollectionPageState,
} from "../../layout/collection-page";
import { ProjectIcon } from "./project-icon";
import { useT } from "../../i18n";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import { useFormatRelativeDate } from "./labels";
import { ProjectStatusBadge, ProjectPriorityBadge } from "./project-badge";
import { ProjectLeadPicker } from "./project-lead-picker";
import { ProjectBatchToolbar, ProjectRowActions, ProjectTable, sortProjects } from "./project-table";
import { PAGE_GUTTER, PAGE_TOOLBAR } from "../../layout/page-header";
import { cn } from "@multica/ui/lib/utils";

// Composite "type:id" lead value so the string[] filter holds member/agent
// refs alike.
function leadFilterValue(p: Project): string | null {
  return p.lead_type && p.lead_id ? `${p.lead_type}:${p.lead_id}` : null;
}

// ---------------------------------------------------------------------------
// Card (comfortable) view — kept from the prior page.
// ---------------------------------------------------------------------------

function ProjectCard({
  project,
  pinned,
  canDelete,
}: {
  project: Project;
  pinned: boolean;
  canDelete: boolean;
}) {
  const { t } = useT("projects");
  const wsPaths = useWorkspacePaths();
  const formatRelativeDate = useFormatRelativeDate();
  const updateProject = useUpdateProject();
  const handleUpdate = useCallback(
    (data: UpdateProjectRequest) => updateProject.mutate({ id: project.id, ...data }),
    [project.id, updateProject],
  );
  const progressPercent =
    project.issue_count > 0
      ? Math.round((project.done_count / project.issue_count) * 100)
      : 0;

  return (
    <div className="group/card group/row flex flex-col rounded-md border bg-card transition-colors hover:border-primary/50">
      <div className="p-3 pb-2">
        <div className="flex items-center gap-2">
          <AppLink
            href={wsPaths.projectDetail(project.id)}
            className="flex min-w-0 flex-1 items-center gap-2"
          >
            <ProjectIcon project={project} size="sm" />
            <h3 className="truncate text-body font-medium">{project.title}</h3>
          </AppLink>
          <ProjectRowActions project={project} pinned={pinned} canDelete={canDelete} />
          <ProjectStatusBadge project={project} handleUpdate={handleUpdate} triggerClassName="shrink-0" />
        </div>

        {project.issue_count > 0 ? (
          <div className="flex items-center justify-end gap-1.5 pt-2">
            <div className="relative h-4 w-4">
              <svg className="h-4 w-4 -rotate-90" viewBox="0 0 16 16">
                <circle className="text-muted" strokeWidth="2" stroke="currentColor" fill="none" r="6" cx="8" cy="8" />
                <circle
                  className="text-emerald-500"
                  strokeWidth="2"
                  stroke="currentColor"
                  fill="none"
                  r="6"
                  cx="8"
                  cy="8"
                  strokeDasharray={`${progressPercent * 0.377} 37.7`}
                  strokeLinecap="round"
                />
              </svg>
            </div>
            <span className="text-micro tabular-nums text-muted-foreground">
              {project.done_count}/{project.issue_count}
            </span>
          </div>
        ) : (
          <span className="flex justify-end pt-2 text-micro text-muted-foreground">
            {t(($) => $.detail.no_issues_yet)}
          </span>
        )}
      </div>

      <div className="mt-0 flex items-center justify-between border-t px-3 pb-3 pt-2">
        <ProjectLeadPicker
          project={project}
          handleUpdate={handleUpdate}
          renderTrigger={(leadName) => (
            <button type="button" className="-mx-1.5 flex items-center gap-1.5 rounded-xs px-1.5 py-0.5 transition-colors hover:bg-accent/60">
              {project.lead_type && project.lead_id ? (
                <ActorAvatar actorType={project.lead_type} actorId={project.lead_id} size="sm" enableHoverCard />
              ) : (
                <span className="inline-flex h-5 w-5 rounded-full border border-dashed border-muted-foreground/30" />
              )}
              <span className="max-w-[60px] truncate text-micro text-muted-foreground">
                {leadName ?? t(($) => $.lead.no_lead)}
              </span>
            </button>
          )}
        />
        <div className="flex items-center gap-2">
          <ProjectPriorityBadge project={project} handleUpdate={handleUpdate} align="start" />
          <span className="text-micro text-muted-foreground">
            {formatRelativeDate(project.created_at)}
          </span>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Toolbar — search + result count + filter + display (compact only) + view
// toggle.
// ---------------------------------------------------------------------------

const STATUS_VALUES: ProjectStatus[] = [
  "planned",
  "in_progress",
  "paused",
  "completed",
  "cancelled",
];
const PRIORITY_VALUES: ProjectPriority[] = ["urgent", "high", "medium", "low", "none"];
const COLUMN_KEYS: ProjectColumnKey[] = ["priority", "progress", "lead", "issues", "created"];
const SORT_FIELDS: ProjectSortField[] = ["name", "priority", "status", "progress", "created"];

function countActiveFilters(f: ProjectListFilters): number {
  let c = 0;
  if (f.statuses.length) c++;
  if (f.priorities.length) c++;
  if (f.leads.length) c++;
  return c;
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export function ProjectsPage() {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const currentUser = useAuthStore((s) => s.user);
  const { getActorName } = useActorName();

  const viewMode = useProjectViewStore((s) => s.viewMode);
  const setViewMode = useProjectViewStore((s) => s.setViewMode);
  const sortField = useProjectViewStore((s) => s.sortField);
  const sortDirection = useProjectViewStore((s) => s.sortDirection);
  const hiddenColumns = useProjectViewStore((s) => s.hiddenColumns);
  const filters = useProjectViewStore((s) => s.filters);
  const setSortField = useProjectViewStore((s) => s.setSortField);
  const setSortDirection = useProjectViewStore((s) => s.setSortDirection);
  const toggleColumn = useProjectViewStore((s) => s.toggleColumn);
  const toggleFilter = useProjectViewStore((s) => s.toggleFilter);
  const clearFilters = useProjectViewStore((s) => s.clearFilters);
  const isCompact = viewMode === "compact";

  const { data: projects = [], isLoading } = useQuery(projectListOptions(wsId));
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: pins = [] } = useQuery({
    ...pinListOptions(wsId, currentUser?.id ?? ""),
    enabled: !!wsId && !!currentUser?.id,
  });
  const openCreateProject = () => useModalStore.getState().open("create-project");

  const isWorkspaceAdmin = useMemo(() => {
    if (!currentUser) return false;
    const me = members.find((m: MemberWithUser) => m.user_id === currentUser.id);
    return me?.role === "owner" || me?.role === "admin";
  }, [members, currentUser]);

  const pinnedProjectIds = useMemo(() => {
    const s = new Set<string>();
    for (const pin of pins) if (pin.item_type === "project") s.add(pin.item_id);
    return s;
  }, [pins]);

  const [search, setSearch] = useState("");
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(new Set());

  const activeFilterCount = countActiveFilters(filters);
  const hasActiveFilters = activeFilterCount > 0;

  // Filter option counts derive from the full set so toggling one dimension
  // doesn't make the others vanish.
  const leadOptions = useMemo(() => {
    const m = new Map<string, { type: string; id: string; count: number }>();
    for (const p of projects) {
      const v = leadFilterValue(p);
      if (!v || !p.lead_type || !p.lead_id) continue;
      const e = m.get(v);
      if (e) e.count += 1;
      else m.set(v, { type: p.lead_type, id: p.lead_id, count: 1 });
    }
    return m;
  }, [projects]);

  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    const filtered = projects.filter((p) => {
      if (q && !p.title.toLowerCase().includes(q) && !matchesPinyin(p.title, q)) {
        return false;
      }
      if (filters.statuses.length && !filters.statuses.includes(p.status)) return false;
      if (filters.priorities.length && !filters.priorities.includes(p.priority)) {
        return false;
      }
      if (filters.leads.length) {
        const v = leadFilterValue(p);
        if (!v || !filters.leads.includes(v)) return false;
      }
      return true;
    });
    return sortProjects(filtered, sortField, sortDirection);
  }, [projects, search, filters, sortField, sortDirection]);

  const selectedProjects = visible.filter((p) => selectedIds.has(p.id));

  const sortLabel = (f: ProjectSortField) =>
    f === "name"
      ? t(($) => $.table.name)
      : f === "priority"
        ? t(($) => $.table.priority)
        : f === "status"
          ? t(($) => $.table.status)
          : f === "progress"
            ? t(($) => $.table.progress)
            : t(($) => $.table.created);
  const columnLabel = (k: ProjectColumnKey) =>
    k === "priority"
      ? t(($) => $.table.priority)
      : k === "progress"
        ? t(($) => $.table.progress)
        : k === "lead"
          ? t(($) => $.table.lead)
          : k === "issues"
            ? t(($) => $.table.issues)
            : t(($) => $.table.created);

  const showEmpty = !isLoading && projects.length === 0;
  const countBadge = (n: number) => (
    <span className="ml-auto pl-3 text-caption text-muted-foreground">{n}</span>
  );

  return (
    // relative: positioning anchor for the page-centered batch toolbar.
    <div className="relative flex flex-1 min-h-0 flex-col">
      <CollectionPageHeader
        icon={FolderKanban}
        title={t(($) => $.page.title)}
        count={projects.length}
        actions={
          <CollectionPageHeaderAction
            icon={Plus}
            label={t(($) => $.page.new_project)}
            onClick={openCreateProject}
          />
        }
      />

      {showEmpty ? (
        <CollectionPageState
          icon={FolderKanban}
          title={t(($) => $.page.empty)}
          actions={
            <Button size="sm" variant="outline" onClick={openCreateProject}>
              {t(($) => $.page.create_first)}
            </Button>
          }
        />
      ) : (
        <>
          {/* Toolbar */}
          <div className={PAGE_TOOLBAR}>
            <div className="flex min-w-0 items-center gap-2">
              <div className="relative hidden md:block">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  aria-label={t(($) => $.page.search_placeholder)}
                  placeholder={t(($) => $.page.search_placeholder)}
                  className="h-8 w-56 pl-8 text-body"
                />
              </div>
              {(hasActiveFilters || search.trim().length > 0) && (
                <span
                  title={t(($) => $.toolbar.result_count_title)}
                  className="hidden shrink-0 text-caption tabular-nums text-muted-foreground md:inline"
                >
                  {visible.length} / {projects.length}
                </span>
              )}
            </div>

            <div className="flex shrink-0 items-center gap-1">
              {/* Filter */}
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button
                      variant={hasActiveFilters ? "default" : "outline"}
                      size="sm"
                      className={
                        hasActiveFilters
                          ? "h-8 w-8 gap-1 bg-brand px-0 text-white hover:bg-brand/90 md:w-auto md:px-2.5"
                          : "h-8 w-8 gap-1 px-0 text-muted-foreground md:w-auto md:px-2.5"
                      }
                    >
                      <Filter className="size-3.5" />
                      {hasActiveFilters ? (
                        <>
                          <span className="hidden md:inline">
                            {t(($) => $.toolbar.filter_active_count, { count: activeFilterCount })}
                          </span>
                          <span className="tabular-nums md:hidden">{activeFilterCount}</span>
                        </>
                      ) : (
                        <span className="hidden md:inline">{t(($) => $.toolbar.filter_label)}</span>
                      )}
                      {hasActiveFilters && (
                        <span
                          role="button"
                          tabIndex={-1}
                          aria-label={t(($) => $.toolbar.clear_filters)}
                          className="-mr-1 ml-0.5 hidden rounded-sm p-0.5 hover:bg-white/20 md:inline-flex"
                          onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            clearFilters();
                          }}
                          onPointerDown={(e) => e.stopPropagation()}
                        >
                          <X className="size-3" />
                        </span>
                      )}
                    </Button>
                  }
                />
                <DropdownMenuContent align="end" className="w-auto">
                  <DropdownMenuSub>
                    <DropdownMenuSubTrigger>
                      <span className="flex-1">{t(($) => $.toolbar.section_status)}</span>
                      {filters.statuses.length > 0 && (
                        <span className="text-caption font-medium text-primary">{filters.statuses.length}</span>
                      )}
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent className="w-auto min-w-44">
                      {STATUS_VALUES.map((s) => (
                        <DropdownMenuCheckboxItem
                          key={s}
                          checked={filters.statuses.includes(s)}
                          onCheckedChange={() => toggleFilter("statuses", s)}
                          className={FILTER_ITEM_CLASS}
                        >
                          <HoverCheck checked={filters.statuses.includes(s)} />
                          {t(($) => $.status[s])}
                        </DropdownMenuCheckboxItem>
                      ))}
                    </DropdownMenuSubContent>
                  </DropdownMenuSub>
                  <DropdownMenuSub>
                    <DropdownMenuSubTrigger>
                      <span className="flex-1">{t(($) => $.toolbar.section_priority)}</span>
                      {filters.priorities.length > 0 && (
                        <span className="text-caption font-medium text-primary">{filters.priorities.length}</span>
                      )}
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent className="w-auto min-w-44">
                      {PRIORITY_VALUES.map((pr) => (
                        <DropdownMenuCheckboxItem
                          key={pr}
                          checked={filters.priorities.includes(pr)}
                          onCheckedChange={() => toggleFilter("priorities", pr)}
                          className={FILTER_ITEM_CLASS}
                        >
                          <HoverCheck checked={filters.priorities.includes(pr)} />
                          {t(($) => $.priority[pr])}
                        </DropdownMenuCheckboxItem>
                      ))}
                    </DropdownMenuSubContent>
                  </DropdownMenuSub>
                  <DropdownMenuSub>
                    <DropdownMenuSubTrigger>
                      <span className="flex-1">{t(($) => $.toolbar.section_lead)}</span>
                      {filters.leads.length > 0 && (
                        <span className="text-caption font-medium text-primary">{filters.leads.length}</span>
                      )}
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent className="max-h-72 w-auto min-w-48 overflow-y-auto">
                      {[...leadOptions.entries()].map(([value, { type, id, count }]) => (
                        <DropdownMenuCheckboxItem
                          key={value}
                          checked={filters.leads.includes(value)}
                          onCheckedChange={() => toggleFilter("leads", value)}
                          className={FILTER_ITEM_CLASS}
                        >
                          <HoverCheck checked={filters.leads.includes(value)} />
                          <ActorAvatar actorType={type} actorId={id} size="sm" />
                          <span className="min-w-0 truncate">{getActorName(type, id)}</span>
                          {countBadge(count)}
                        </DropdownMenuCheckboxItem>
                      ))}
                    </DropdownMenuSubContent>
                  </DropdownMenuSub>
                </DropdownMenuContent>
              </DropdownMenu>

              {/* Display (sort + columns). Always present — view mode is a
                  pure presentation choice and must not reshape the toolbar.
                  Sort applies to both views; the columns section is shown
                  only in the table view (cards have no columns). */}
              <Popover>
                  <Tooltip>
                    <PopoverTrigger
                      render={
                        <TooltipTrigger
                          render={
                            <Button variant="outline" size="sm" className="h-8 w-8 gap-1 px-0 text-muted-foreground md:w-auto md:px-2.5">
                              {sortDirection === "asc" ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />}
                              <span className="hidden md:inline">{sortLabel(sortField)}</span>
                            </Button>
                          }
                        />
                      }
                    />
                    <TooltipContent side="bottom">{t(($) => $.toolbar.display)}</TooltipContent>
                  </Tooltip>
                  <PopoverContent align="end" className="w-64 p-0">
                    <div className="border-b px-3 py-2.5">
                      <span className="text-caption font-medium text-muted-foreground">{t(($) => $.toolbar.sort_by)}</span>
                      <div className="mt-2 flex items-center gap-1.5">
                        <DropdownMenu>
                          <DropdownMenuTrigger
                            render={
                              <Button variant="outline" size="sm" className="flex-1 justify-between text-caption">
                                {sortLabel(sortField)}
                                <ChevronDown className="size-3 text-muted-foreground" />
                              </Button>
                            }
                          />
                          <DropdownMenuContent align="start" className="w-auto">
                            <DropdownMenuRadioGroup
                              value={sortField}
                              onValueChange={(v) => setSortField(v as ProjectSortField)}
                            >
                              {SORT_FIELDS.map((f) => (
                                <DropdownMenuRadioItem key={f} value={f}>
                                  {sortLabel(f)}
                                </DropdownMenuRadioItem>
                              ))}
                            </DropdownMenuRadioGroup>
                          </DropdownMenuContent>
                        </DropdownMenu>
                        <Button
                          variant="outline"
                          size="icon-sm"
                          onClick={() => setSortDirection(sortDirection === "asc" ? "desc" : "asc")}
                          title={sortDirection === "asc" ? t(($) => $.toolbar.direction_asc) : t(($) => $.toolbar.direction_desc)}
                        >
                          {sortDirection === "asc" ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />}
                        </Button>
                      </div>
                    </div>
                    {isCompact && (
                      <div className="px-3 py-2.5">
                        <span className="text-caption font-medium text-muted-foreground">{t(($) => $.toolbar.section_columns)}</span>
                        <div className="mt-2 space-y-2">
                          {COLUMN_KEYS.map((key) => (
                            <label key={key} className="flex cursor-pointer items-center justify-between">
                              <span className="text-body">{columnLabel(key)}</span>
                              <Switch size="sm" checked={!hiddenColumns.includes(key)} onCheckedChange={() => toggleColumn(key)} />
                            </label>
                          ))}
                        </div>
                      </div>
                    )}
                  </PopoverContent>
                </Popover>

              {/* View selector — a dropdown menu to pick the list view,
                  aligned with the issue list's view menu. The trigger shows
                  the active view; the menu carries every mode so new views
                  can be added as menu items. Pure presentation. */}
              <DropdownMenu>
                <Tooltip>
                  <DropdownMenuTrigger
                    render={
                      <TooltipTrigger
                        render={
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-8 w-8 gap-1 px-0 text-muted-foreground md:w-auto md:px-2.5"
                          >
                            {isCompact ? (
                              <Rows3 className="size-3.5" />
                            ) : (
                              <LayoutGrid className="size-3.5" />
                            )}
                            <span className="hidden md:inline">
                              {isCompact ? t(($) => $.page.view_table) : t(($) => $.page.view_cards)}
                            </span>
                          </Button>
                        }
                      />
                    }
                  />
                  <TooltipContent side="bottom">{t(($) => $.toolbar.view)}</TooltipContent>
                </Tooltip>
                <DropdownMenuContent align="end" className="w-auto">
                  <DropdownMenuGroup>
                    <DropdownMenuLabel>{t(($) => $.toolbar.view)}</DropdownMenuLabel>
                  </DropdownMenuGroup>
                  <DropdownMenuRadioGroup
                    value={viewMode}
                    onValueChange={(v) => setViewMode(v as ProjectViewMode)}
                  >
                    <DropdownMenuRadioItem value="compact">
                      <Rows3 />
                      {t(($) => $.page.view_table)}
                    </DropdownMenuRadioItem>
                    <DropdownMenuRadioItem value="comfortable">
                      <LayoutGrid />
                      {t(($) => $.page.view_cards)}
                    </DropdownMenuRadioItem>
                  </DropdownMenuRadioGroup>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>

          {/* Body */}
          {isLoading ? (
            <LoadingState isCompact={isCompact} />
          ) : visible.length === 0 ? (
            <div className="flex flex-1 flex-col items-center justify-center py-24 text-muted-foreground">
              <Search className="mb-3 h-10 w-10 opacity-30" />
              <p className="text-body">{t(($) => $.page.no_matches)}</p>
            </div>
          ) : isCompact ? (
            <ProjectTable
              projects={visible}
              pinnedIds={pinnedProjectIds}
              canDelete={isWorkspaceAdmin}
              selectedIds={selectedIds}
              onSelectedIdsChange={setSelectedIds}
            />
          ) : (
            <div className={cn("min-h-0 flex-1 overflow-y-auto pt-4", PAGE_GUTTER)}>
              <div
                className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4"
                style={{ paddingBottom: LIST_GRID_BOTTOM_CLEARANCE }}
              >
                {visible.map((project) => (
                  <ProjectCard
                    key={project.id}
                    project={project}
                    pinned={pinnedProjectIds.has(project.id)}
                    canDelete={isWorkspaceAdmin}
                  />
                ))}
              </div>
            </div>
          )}

          <ProjectBatchToolbar
            rows={selectedProjects}
            pinnedIds={pinnedProjectIds}
            canDelete={isWorkspaceAdmin}
            onClear={() => setSelectedIds(new Set())}
          />
        </>
      )}
    </div>
  );
}

function LoadingState({ isCompact }: { isCompact: boolean }) {
  if (isCompact) {
    return (
      <div className={cn("min-h-0 flex-1 overflow-auto pt-4", PAGE_GUTTER)}>
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-11 w-full rounded-md" />
          ))}
        </div>
      </div>
    );
  }
  return (
    <div className={cn("grid grid-cols-1 gap-3 pt-4 sm:grid-cols-2 lg:grid-cols-4", PAGE_GUTTER)}>
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="flex flex-col gap-2 rounded-md border p-3">
          <div className="flex items-center gap-2">
            <Skeleton className="h-8 w-8 rounded-xs" />
            <Skeleton className="h-4 w-3/4" />
          </div>
          <div className="flex gap-1.5">
            <Skeleton className="h-5 w-16 rounded-xs" />
            <Skeleton className="h-5 w-20 rounded-xs" />
          </div>
        </div>
      ))}
    </div>
  );
}
