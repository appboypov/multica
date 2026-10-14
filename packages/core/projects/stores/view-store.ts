"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { createWorkspaceAwareStorage, registerForWorkspaceRehydration } from "../../platform/workspace-storage";
import { defaultStorage } from "../../platform/storage";

// Projects keeps a compact table and comfortable card grid. Sort and filters
// feed both views; hidden columns apply only to the table.
export type ProjectViewMode = "compact" | "comfortable";

export type ProjectSortField = "name" | "priority" | "progress" | "created";

export type ProjectSortDirection = "asc" | "desc";

export const PROJECT_SORT_DEFAULT_DIRECTION: Record<
  ProjectSortField,
  ProjectSortDirection
> = {
  name: "asc",
  priority: "desc",
  progress: "desc",
  created: "desc",
};

/** Multi-select filters. Empty array per dimension = inactive. */
export interface ProjectListFilters {
  /** ProjectPriority values. */
  priorities: string[];
  /** Composite "type:id" lead refs (member or agent). */
  leads: string[];
}

export const EMPTY_PROJECT_FILTERS: ProjectListFilters = {
  priorities: [],
  leads: [],
};

// Name is always visible; the other table columns can be hidden.
export type ProjectColumnKey = "priority" | "progress" | "lead" | "issues" | "created";

/** Issues count is opt-in; the rest show by default (matching the prior
 *  compact table). */
export const PROJECT_DEFAULT_HIDDEN_COLUMNS: ProjectColumnKey[] = ["issues"];

export interface ProjectViewState {
  viewMode: ProjectViewMode;
  sortField: ProjectSortField;
  sortDirection: ProjectSortDirection;
  hiddenColumns: ProjectColumnKey[];
  filters: ProjectListFilters;
  collapsedOrganisations: string[];
  /** Repository rows folded open to their worklanes, on Projects and an organisation's page. */
  expandedRepositories: string[];
  setViewMode: (mode: ProjectViewMode) => void;
  toggleSort: (field: ProjectSortField) => void;
  setSortField: (field: ProjectSortField) => void;
  setSortDirection: (direction: ProjectSortDirection) => void;
  toggleColumn: (key: ProjectColumnKey) => void;
  toggleOrganisation: (id: string) => void;
  toggleRepository: (id: string) => void;
  toggleFilter: (key: keyof ProjectListFilters, value: string) => void;
  clearFilters: () => void;
}

const DEFAULTS = {
  viewMode: "compact" as ProjectViewMode,
  sortField: "created" as ProjectSortField,
  sortDirection: PROJECT_SORT_DEFAULT_DIRECTION.created,
  hiddenColumns: PROJECT_DEFAULT_HIDDEN_COLUMNS,
  filters: EMPTY_PROJECT_FILTERS,
  collapsedOrganisations: [] as string[],
  expandedRepositories: [] as string[],
};

export const useProjectViewStore = create<ProjectViewState>()(
  persist(
    (set) => ({
      ...DEFAULTS,
      setViewMode: (mode) => set({ viewMode: mode }),
      toggleSort: (field) =>
        set((state) =>
          state.sortField === field
            ? { sortDirection: state.sortDirection === "asc" ? "desc" : "asc" }
            : {
                sortField: field,
                sortDirection: PROJECT_SORT_DEFAULT_DIRECTION[field],
              },
        ),
      setSortField: (field) =>
        set((state) =>
          state.sortField === field
            ? {}
            : {
                sortField: field,
                sortDirection: PROJECT_SORT_DEFAULT_DIRECTION[field],
              },
        ),
      setSortDirection: (direction) => set({ sortDirection: direction }),
      toggleColumn: (key) =>
        set((state) => ({
          hiddenColumns: state.hiddenColumns.includes(key)
            ? state.hiddenColumns.filter((k) => k !== key)
            : [...state.hiddenColumns, key],
        })),
      toggleOrganisation: (id) =>
        set((state) => ({
          collapsedOrganisations: state.collapsedOrganisations.includes(id)
            ? state.collapsedOrganisations.filter((value) => value !== id)
            : [...state.collapsedOrganisations, id],
        })),
      toggleRepository: (id) =>
        set((state) => ({
          expandedRepositories: state.expandedRepositories.includes(id)
            ? state.expandedRepositories.filter((value) => value !== id)
            : [...state.expandedRepositories, id],
        })),
      toggleFilter: (key, value) =>
        set((state) => {
          const list = state.filters[key] as string[];
          const next = list.includes(value)
            ? list.filter((v) => v !== value)
            : [...list, value];
          return { filters: { ...state.filters, [key]: next } };
        }),
      clearFilters: () => set({ filters: EMPTY_PROJECT_FILTERS }),
    }),
    {
      name: "multica_projects_view",
      storage: createJSONStorage(() => createWorkspaceAwareStorage(defaultStorage)),
      partialize: (state) => ({
        viewMode: state.viewMode,
        sortField: state.sortField,
        sortDirection: state.sortDirection,
        hiddenColumns: state.hiddenColumns,
        filters: state.filters,
        collapsedOrganisations: state.collapsedOrganisations,
        expandedRepositories: state.expandedRepositories,
      }),
      // Keep only supported filters and sorts when loading saved preferences.
      merge: (persisted, current) => {
        if (!persisted) return { ...current, ...DEFAULTS };
        const p = persisted as Partial<ProjectViewState>;
        const sortField = p.sortField && Object.hasOwn(PROJECT_SORT_DEFAULT_DIRECTION, p.sortField)
          ? p.sortField
          : DEFAULTS.sortField;
        return {
          ...current,
          ...p,
          sortField,
          sortDirection: sortField === p.sortField
            ? p.sortDirection ?? DEFAULTS.sortDirection
            : DEFAULTS.sortDirection,
          collapsedOrganisations: p.collapsedOrganisations ?? DEFAULTS.collapsedOrganisations,
          expandedRepositories: p.expandedRepositories ?? DEFAULTS.expandedRepositories,
          filters: {
            priorities: p.filters?.priorities ?? [],
            leads: p.filters?.leads ?? [],
          },
        };
      },
    }
  )
);

registerForWorkspaceRehydration(() => useProjectViewStore.persist.rehydrate());
