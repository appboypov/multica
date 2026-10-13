"use client";

import { useCallback, useMemo, useState, type MouseEvent } from "react";
import {
  Building2,
  ChevronRight,
  ExternalLink,
  MoreHorizontal,
  Pin,
  PinOff,
  Trash2,
  X,
} from "lucide-react";
import { toast } from "sonner";
import {
  useDeleteProject,
  useProjectViewStore,
  useUpdateProject,
  type ProjectColumnKey,
  type ProjectOrganisationGroup,
  type ProjectSortDirection,
  type ProjectSortField,
} from "@multica/core/projects";
import { useCreatePin, useDeletePin } from "@multica/core/pins";
import { useWorkspacePaths } from "@multica/core/paths";
import type {
  Project,
  ProjectPriority,
  UpdateProjectRequest,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import {
  ListGrid,
  ListGridCell,
  ListGridHeader,
  ListGridHeaderCell,
  ListGridRow,
  LIST_GRID_BOTTOM_CLEARANCE,
} from "@multica/ui/components/ui/list-grid";
import { cn } from "@multica/ui/lib/utils";
import { AppLink, useIntentNavigate, useRowLink } from "../../navigation";
import { ActorAvatar } from "../../common/actor-avatar";
import { useT } from "../../i18n";
import { ProjectIcon } from "./project-icon";
import { useFormatRelativeDate } from "./labels";
import { ProjectPriorityBadge } from "./project-badge";
import { ProjectLeadPicker } from "./project-lead-picker";

// The fold key of the "No organisation" group in `collapsedOrganisations`.
const NO_ORGANISATION_KEY = "none";

// Sort order map for the priority column (header sort needs a total order).
const PRIORITY_ORDER: Record<ProjectPriority, number> = {
  urgent: 4,
  high: 3,
  medium: 2,
  low: 1,
  none: 0,
};

const progressOf = (p: Project) =>
  p.issue_count > 0 ? p.done_count / p.issue_count : -1;

/**
 * Orders projects by the view store's sort. Callers sort before they group,
 * because `groupProjectsByOrganisation` keeps the input order in each group.
 */
export function sortProjects(
  projects: readonly Project[],
  field: ProjectSortField,
  direction: ProjectSortDirection,
): Project[] {
  const dir = direction === "asc" ? 1 : -1;
  return [...projects].sort((a, b) => {
    if (field === "name") return a.title.localeCompare(b.title) * dir;
    if (field === "priority") {
      return (
        (PRIORITY_ORDER[a.priority] - PRIORITY_ORDER[b.priority]) * dir ||
        a.title.localeCompare(b.title)
      );
    }
    if (field === "progress") {
      return (progressOf(a) - progressOf(b)) * dir || a.title.localeCompare(b.title);
    }
    return (Date.parse(a.created_at) - Date.parse(b.created_at)) * dir;
  });
}

// ---------------------------------------------------------------------------
// Table (compact) view — ListGrid. Name is the core column;
// priority/progress/lead/issues/created collapse below @2xl, with min-width
// + the wrapper's overflow as the escape valve. Rows use whole-row mouse
// navigation; inline controls stop propagation so edit/menu clicks stay local.
// ---------------------------------------------------------------------------

const COLUMN_WIDTHS: Record<ProjectColumnKey, number> = {
  priority: 116,
  progress: 88,
  lead: 132,
  issues: 80,
  created: 104,
};

// Fixed tracks: edges 12+12, checkbox 16, name min 200, kebab 28 = 268,
// plus the 9 gap-x-3 gaps between the wide template's 10 tracks.
const FIXED_TRACKS_WIDTH = 268 + 9 * 12;

// Render/track order: checkbox, name, priority, progress, lead, issues,
// created, kebab. MUST be a literal string — Tailwind can't see interpolated
// `grid-cols-[...]` arbitrary values, so an interpolated width silently drops
// the whole template and the grid collapses to one column.
const GRID_COLS =
  "grid-cols-[0.75rem_1rem_minmax(120px,1fr)_1.75rem_0.75rem] " +
  "@2xl:grid-cols-[0.75rem_1rem_minmax(200px,1fr)_var(--pjc-priority)_var(--pjc-progress)_var(--pjc-lead)_var(--pjc-issues)_var(--pjc-created)_1.75rem_0.75rem]";

const stopRowNavigation = (e: MouseEvent) => e.stopPropagation();

function columnTrackVars(
  isVisible: (key: ProjectColumnKey) => boolean,
): React.CSSProperties {
  const width = (key: ProjectColumnKey) =>
    isVisible(key) ? `${COLUMN_WIDTHS[key]}px` : "0px";
  const minWidth =
    FIXED_TRACKS_WIDTH +
    (Object.keys(COLUMN_WIDTHS) as ProjectColumnKey[]).reduce(
      (sum, key) => sum + (isVisible(key) ? COLUMN_WIDTHS[key] : 0),
      0,
    );
  return {
    "--pjc-priority": width("priority"),
    "--pjc-progress": width("progress"),
    "--pjc-lead": width("lead"),
    "--pjc-issues": width("issues"),
    "--pjc-created": width("created"),
    "--pjc-minw": `${minWidth}px`,
  } as React.CSSProperties;
}

function ProgressRing({ project }: { project: Project }) {
  if (project.issue_count === 0) {
    return <span className="text-caption text-faint-foreground">—</span>;
  }
  const pct = Math.round((project.done_count / project.issue_count) * 100);
  return (
    <span className="flex items-center gap-1.5">
      <span className="relative h-3.5 w-3.5">
        <svg className="h-3.5 w-3.5 -rotate-90" viewBox="0 0 16 16">
          <circle className="text-muted" strokeWidth="2" stroke="currentColor" fill="none" r="6" cx="8" cy="8" />
          <circle
            className="text-emerald-500"
            strokeWidth="2"
            stroke="currentColor"
            fill="none"
            r="6"
            cx="8"
            cy="8"
            strokeDasharray={`${pct * 0.377} 37.7`}
            strokeLinecap="round"
          />
        </svg>
      </span>
      <span className="text-caption tabular-nums text-muted-foreground">
        {project.done_count}/{project.issue_count}
      </span>
    </span>
  );
}

// Compact rows own whole-row navigation; callers stop propagation around this
// menu so action clicks do not bubble into the rowLink handler.
export function ProjectRowActions({
  project,
  pinned,
  canDelete,
}: {
  project: Project;
  pinned: boolean;
  canDelete: boolean;
}) {
  const { t } = useT("projects");
  const { t: tCommon } = useT("common");
  const wsPaths = useWorkspacePaths();
  const intentNavigate = useIntentNavigate();
  const createPin = useCreatePin();
  const deletePin = useDeletePin();
  const deleteProject = useDeleteProject();
  const [deleteOpen, setDeleteOpen] = useState(false);

  const togglePin = () => {
    if (pinned) deletePin.mutate({ itemType: "project", itemId: project.id });
    else createPin.mutate({ item_type: "project", item_id: project.id });
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <button
              type="button"
              aria-label={t(($) => $.page.row_menu)}
              className="flex size-7 items-center justify-center rounded-md text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-accent-foreground group-hover/row:opacity-100 data-popup-open:bg-accent data-popup-open:opacity-100 data-popup-open:text-accent-foreground"
            >
              <MoreHorizontal className="size-4" />
            </button>
          }
        />
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem
            onClick={() =>
              intentNavigate(
                wsPaths.projectDetail(project.id),
                "foreground-tab",
                project.title,
              )
            }
          >
            <ExternalLink className="size-3.5" />
            {tCommon(($) => $.navigation.open_in_new_tab)}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={togglePin}>
            {pinned ? (
              <PinOff className="size-3.5" />
            ) : (
              <Pin className="size-3.5" />
            )}
            {pinned ? t(($) => $.page.unpin) : t(($) => $.page.pin)}
          </DropdownMenuItem>
          {canDelete && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onClick={() => setDeleteOpen(true)}
              >
                <Trash2 className="size-3.5" />
                {t(($) => $.page.delete)}
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(($) => $.delete_dialog.title)}</DialogTitle>
            <DialogDescription>
              {t(($) => $.delete_dialog.description)}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setDeleteOpen(false)}
            >
              {t(($) => $.delete_dialog.cancel)}
            </Button>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              onClick={() => {
                deleteProject.mutate(project.id, {
                  onError: (err) =>
                    toast.error(
                      err instanceof Error ? err.message : String(err),
                    ),
                });
                setDeleteOpen(false);
              }}
            >
              {t(($) => $.delete_dialog.confirm)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function CheckboxCell({
  checked,
  onToggle,
}: {
  checked: boolean;
  onToggle: () => void;
}) {
  return (
    <ListGridCell className="justify-center px-0">
      <button
        type="button"
        aria-pressed={checked}
        onClick={(e) => {
          stopRowNavigation(e);
          onToggle();
        }}
        onAuxClick={stopRowNavigation}
        className={`-m-1.5 flex items-center p-1.5 ${
          checked ? "" : "opacity-0 transition-opacity group-hover/row:opacity-100"
        }`}
      >
        <Checkbox checked={checked} tabIndex={-1} className="pointer-events-none" />
      </button>
    </ListGridCell>
  );
}

function ProjectTableRow({
  project,
  pinned,
  canDelete,
  isColVisible,
  selected,
  onToggleSelect,
}: {
  project: Project;
  pinned: boolean;
  canDelete: boolean;
  isColVisible: (key: ProjectColumnKey) => boolean;
  selected: boolean;
  onToggleSelect: (id: string) => void;
}) {
  const wsPaths = useWorkspacePaths();
  const rowLink = useRowLink();
  const formatRelativeDate = useFormatRelativeDate();
  const updateProject = useUpdateProject();
  const handleUpdate = useCallback(
    (data: UpdateProjectRequest) => updateProject.mutate({ id: project.id, ...data }),
    [project.id, updateProject],
  );

  return (
    <ListGridRow
      className={`h-11 cursor-pointer ${selected ? "bg-accent/30" : ""}`}
      {...rowLink(wsPaths.projectDetail(project.id), project.title)}
    >
      <CheckboxCell checked={selected} onToggle={() => onToggleSelect(project.id)} />
      <ListGridCell className="gap-2">
        <ProjectIcon project={project} size="sm" />
        <span className="min-w-0 truncate text-body font-medium">
          {project.title}
        </span>
      </ListGridCell>

      {isColVisible("priority") ? (
        <ListGridCell className="hidden @2xl:flex" onClick={stopRowNavigation} onAuxClick={stopRowNavigation}>
          <ProjectPriorityBadge project={project} handleUpdate={handleUpdate} align="start" />
        </ListGridCell>
      ) : (
        <ListGridCell className="hidden px-0 @2xl:flex" />
      )}

      {isColVisible("progress") ? (
        <ListGridCell className="hidden @2xl:flex">
          <ProgressRing project={project} />
        </ListGridCell>
      ) : (
        <ListGridCell className="hidden px-0 @2xl:flex" />
      )}

      {isColVisible("lead") ? (
        <ListGridCell className="hidden @2xl:flex" onClick={stopRowNavigation} onAuxClick={stopRowNavigation}>
          <ProjectLeadPicker
            project={project}
            handleUpdate={handleUpdate}
            align="start"
            renderTrigger={(leadName) => (
              <button
                type="button"
                className="flex min-w-0 items-center gap-1.5 rounded-xs px-1 py-0.5 transition-colors hover:bg-accent/60"
              >
                {project.lead_type && project.lead_id ? (
                  <ActorAvatar actorType={project.lead_type} actorId={project.lead_id} size="sm" enableHoverCard />
                ) : (
                  <span className="inline-flex h-[18px] w-[18px] rounded-full border border-dashed border-muted-foreground/30" />
                )}
                <span className="min-w-0 truncate text-caption text-muted-foreground">
                  {leadName ?? "—"}
                </span>
              </button>
            )}
          />
        </ListGridCell>
      ) : (
        <ListGridCell className="hidden px-0 @2xl:flex" />
      )}

      {isColVisible("issues") ? (
        <ListGridCell className="hidden justify-end font-mono text-caption tabular-nums text-muted-foreground @2xl:flex">
          {project.issue_count}
        </ListGridCell>
      ) : (
        <ListGridCell className="hidden px-0 @2xl:flex" />
      )}

      {isColVisible("created") ? (
        <ListGridCell className="hidden whitespace-nowrap text-caption tabular-nums text-muted-foreground @2xl:flex">
          {formatRelativeDate(project.created_at)}
        </ListGridCell>
      ) : (
        <ListGridCell className="hidden px-0 @2xl:flex" />
      )}

      <ListGridCell className="justify-end px-0">
        <span onClick={stopRowNavigation} onAuxClick={stopRowNavigation} className="flex items-center">
          <ProjectRowActions project={project} pinned={pinned} canDelete={canDelete} />
        </span>
      </ListGridCell>
    </ListGridRow>
  );
}

function ProjectTableHeader({
  sortField,
  sortDirection,
  onSort,
  isColVisible,
  allSelected,
  someSelected,
  onToggleAll,
}: {
  sortField: ProjectSortField;
  sortDirection: ProjectSortDirection;
  onSort: (field: ProjectSortField) => void;
  isColVisible: (key: ProjectColumnKey) => boolean;
  allSelected: boolean;
  someSelected: boolean;
  onToggleAll: () => void;
}) {
  const { t } = useT("projects");
  const sorted = (field: ProjectSortField) =>
    sortField === field ? sortDirection : false;
  const anySelected = allSelected || someSelected;
  return (
    <ListGridHeader>
      <div className="flex items-center justify-center">
        <button
          type="button"
          aria-pressed={allSelected}
          onClick={onToggleAll}
          className={`-m-1.5 flex items-center p-1.5 ${
            anySelected ? "" : "opacity-0 transition-opacity group-hover/header:opacity-100"
          }`}
        >
          <Checkbox
            checked={allSelected}
            indeterminate={someSelected && !allSelected}
            tabIndex={-1}
            className="pointer-events-none"
          />
        </button>
      </div>
      <ListGridHeaderCell sorted={sorted("name")} onSort={() => onSort("name")}>
        {t(($) => $.table.name)}
      </ListGridHeaderCell>
      {isColVisible("priority") ? (
        <ListGridHeaderCell
          className="hidden @2xl:flex"
          sorted={sorted("priority")}
          onSort={() => onSort("priority")}
        >
          {t(($) => $.table.priority)}
        </ListGridHeaderCell>
      ) : (
        <ListGridHeaderCell className="hidden px-0 @2xl:flex" />
      )}
      {isColVisible("progress") ? (
        <ListGridHeaderCell
          className="hidden @2xl:flex"
          sorted={sorted("progress")}
          onSort={() => onSort("progress")}
        >
          {t(($) => $.table.progress)}
        </ListGridHeaderCell>
      ) : (
        <ListGridHeaderCell className="hidden px-0 @2xl:flex" />
      )}
      {isColVisible("lead") ? (
        <ListGridHeaderCell className="hidden @2xl:flex">
          {t(($) => $.table.lead)}
        </ListGridHeaderCell>
      ) : (
        <ListGridHeaderCell className="hidden px-0 @2xl:flex" />
      )}
      {isColVisible("issues") ? (
        <ListGridHeaderCell className="hidden justify-end @2xl:flex" align="right">
          {t(($) => $.table.issues)}
        </ListGridHeaderCell>
      ) : (
        <ListGridHeaderCell className="hidden px-0 @2xl:flex" />
      )}
      {isColVisible("created") ? (
        <ListGridHeaderCell
          className="hidden @2xl:flex"
          sorted={sorted("created")}
          onSort={() => onSort("created")}
        >
          {t(($) => $.table.created)}
        </ListGridHeaderCell>
      ) : (
        <ListGridHeaderCell className="hidden px-0 @2xl:flex" />
      )}
      <span aria-hidden="true" />
    </ListGridHeader>
  );
}

// A group row spans every track: the chevron folds the group, the name opens
// the organisation's page ("No organisation" has none), then the count.
function OrganisationGroupRow({
  group,
  collapsed,
  onToggle,
}: {
  group: ProjectOrganisationGroup;
  collapsed: boolean;
  onToggle: (key: string) => void;
}) {
  const { t } = useT("projects");
  const wsPaths = useWorkspacePaths();
  const { organisation } = group;
  const name = organisation?.name ?? t(($) => $.organisation.none);
  const foldKey = organisation?.id ?? NO_ORGANISATION_KEY;

  return (
    <div
      role="row"
      className="col-span-full flex h-10 min-w-0 items-center border-b px-3"
    >
      <div role="cell" className="flex min-w-0 items-center gap-1.5">
        <button
          type="button"
          aria-expanded={!collapsed}
          aria-label={t(($) => $.group.toggle_aria, { name })}
          onClick={() => onToggle(foldKey)}
          className="-ml-1 flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <ChevronRight
            aria-hidden="true"
            className={cn(
              "size-3.5 transition-transform motion-reduce:transition-none",
              !collapsed && "rotate-90",
            )}
          />
        </button>
        <Building2 aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
        {organisation ? (
          <AppLink
            href={wsPaths.organisationDetail(organisation.id)}
            className="min-w-0 truncate text-caption font-medium hover:underline"
          >
            {name}
          </AppLink>
        ) : (
          <span className="min-w-0 truncate text-caption font-medium">{name}</span>
        )}
        <span className="shrink-0 text-caption text-muted-foreground">
          {t(($) => $.group.project_count, { count: group.projects.length })}
        </span>
      </div>
    </div>
  );
}

/**
 * The project table of Projects and an organisation's page. `projects` is the
 * sorted, filtered list the header's select-all covers; `groups`, when given,
 * splits it into organisation groups that fold through the view store's
 * `collapsedOrganisations`. Without `groups` the rows render flat.
 */
export function ProjectTable({
  projects,
  groups,
  pinnedIds,
  canDelete,
  selectedIds,
  onSelectedIdsChange,
}: {
  projects: readonly Project[];
  groups?: readonly ProjectOrganisationGroup[];
  pinnedIds: ReadonlySet<string>;
  canDelete: boolean;
  selectedIds: ReadonlySet<string>;
  onSelectedIdsChange: (ids: ReadonlySet<string>) => void;
}) {
  const sortField = useProjectViewStore((s) => s.sortField);
  const sortDirection = useProjectViewStore((s) => s.sortDirection);
  const hiddenColumns = useProjectViewStore((s) => s.hiddenColumns);
  const collapsedOrganisations = useProjectViewStore((s) => s.collapsedOrganisations);
  const toggleSort = useProjectViewStore((s) => s.toggleSort);
  const toggleOrganisation = useProjectViewStore((s) => s.toggleOrganisation);

  const isColVisible = useCallback(
    (key: ProjectColumnKey) => !hiddenColumns.includes(key),
    [hiddenColumns],
  );
  const trackVars = useMemo(() => columnTrackVars(isColVisible), [isColVisible]);
  const collapsed = useMemo(
    () => new Set(collapsedOrganisations),
    [collapsedOrganisations],
  );

  const selectedCount = projects.reduce(
    (n, p) => (selectedIds.has(p.id) ? n + 1 : n),
    0,
  );
  const allSelected = projects.length > 0 && selectedCount === projects.length;
  const someSelected = selectedCount > 0 && !allSelected;
  const handleToggleAll = () =>
    onSelectedIdsChange(allSelected ? new Set() : new Set(projects.map((p) => p.id)));
  const handleToggleSelect = useCallback(
    (id: string) => {
      const next = new Set(selectedIds);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      onSelectedIdsChange(next);
    },
    [selectedIds, onSelectedIdsChange],
  );

  const renderRow = (project: Project) => (
    <ProjectTableRow
      key={project.id}
      project={project}
      pinned={pinnedIds.has(project.id)}
      canDelete={canDelete}
      isColVisible={isColVisible}
      selected={selectedIds.has(project.id)}
      onToggleSelect={handleToggleSelect}
    />
  );

  return (
    <div className="min-h-0 flex-1 overflow-auto @container">
      <ListGrid
        className={`${GRID_COLS} @2xl:min-w-[var(--pjc-minw)]`}
        style={{ ...trackVars, paddingBottom: LIST_GRID_BOTTOM_CLEARANCE }}
      >
        <ProjectTableHeader
          sortField={sortField}
          sortDirection={sortDirection}
          onSort={toggleSort}
          isColVisible={isColVisible}
          allSelected={allSelected}
          someSelected={someSelected}
          onToggleAll={handleToggleAll}
        />
        {groups
          ? groups.map((group) => {
              const key = group.organisation?.id ?? NO_ORGANISATION_KEY;
              const isCollapsed = collapsed.has(key);
              return (
                <div key={key} role="rowgroup" className="col-span-full grid grid-cols-subgrid">
                  <OrganisationGroupRow
                    group={group}
                    collapsed={isCollapsed}
                    onToggle={toggleOrganisation}
                  />
                  {!isCollapsed && group.projects.map(renderRow)}
                </div>
              );
            })
          : projects.map(renderRow)}
      </ListGrid>
    </div>
  );
}

// Batch toolbar — page-anchored (not viewport). Pin all selected (any
// member) + Delete (workspace admin). Mirrors the other lists.
export function ProjectBatchToolbar({
  rows,
  pinnedIds,
  canDelete,
  onClear,
}: {
  rows: Project[];
  pinnedIds: ReadonlySet<string>;
  canDelete: boolean;
  onClear: () => void;
}) {
  const { t } = useT("projects");
  const createPin = useCreatePin();
  const deleteProject = useDeleteProject();
  const [confirmDelete, setConfirmDelete] = useState(false);

  if (rows.length === 0) return null;
  const anyUnpinned = rows.some((p) => !pinnedIds.has(p.id));

  return (
    <>
      <div className="absolute bottom-6 left-1/2 z-50 flex -translate-x-1/2 items-center gap-1 rounded-lg border bg-background px-2 py-1.5 shadow-lg max-md:above-chat-launcher">
        <div className="mr-1 flex items-center gap-1.5 border-r pl-1 pr-2">
          <span className="text-body font-medium">
            {t(($) => $.page.selected, { count: rows.length })}
          </span>
          <button
            type="button"
            aria-label={t(($) => $.page.clear_selection)}
            onClick={onClear}
            className="rounded-xs p-0.5 transition-colors hover:bg-accent"
          >
            <X className="size-3.5 text-muted-foreground" />
          </button>
        </div>
        {anyUnpinned && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              for (const p of rows) {
                if (!pinnedIds.has(p.id)) {
                  createPin.mutate({ item_type: "project", item_id: p.id });
                }
              }
              onClear();
            }}
          >
            <Pin className="mr-1 size-3.5" />
            {t(($) => $.page.pin)}
          </Button>
        )}
        {canDelete && (
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive hover:text-destructive"
            onClick={() => setConfirmDelete(true)}
          >
            <Trash2 className="mr-1 size-3.5" />
            {t(($) => $.page.delete)}
          </Button>
        )}
      </div>

      <Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(($) => $.delete_dialog.title)}</DialogTitle>
            <DialogDescription>{t(($) => $.delete_dialog.description)}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" size="sm" onClick={() => setConfirmDelete(false)}>
              {t(($) => $.delete_dialog.cancel)}
            </Button>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              onClick={() => {
                for (const p of rows) deleteProject.mutate(p.id);
                setConfirmDelete(false);
                onClear();
              }}
            >
              {t(($) => $.delete_dialog.confirm)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
