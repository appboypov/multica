// @vitest-environment node
import { describe, expect, it } from "vitest";
import { createStore } from "zustand/vanilla";
import {
  mergeViewStatePersisted,
  viewStorePersistOptions,
  viewStoreSlice,
  type IssueViewState,
} from "./view-store";

describe("issue view persistence", () => {
  it.each([
    { projectStatusFilters: ["in_progress", "completed"] },
    { projectStatusFilters: "paused" },
    { projectStatusFilters: null },
  ])(
    "drops a saved project lifecycle filter (%j) without losing the other filters",
    ({ projectStatusFilters }) => {
      const current = createStore<IssueViewState>()(viewStoreSlice).getState();
      const saved = {
        projectStatusFilters,
        projectFilters: ["project-1"],
        statusFilters: ["in_progress"],
        priorityFilters: ["high"],
        includeNoProject: true,
      };
      const merged = mergeViewStatePersisted(saved, current);
      expect(merged).not.toHaveProperty("projectStatusFilters");
      expect(merged).not.toHaveProperty("toggleProjectStatusFilter");
      expect(merged.projectFilters).toEqual(["project-1"]);
      expect(merged.statusFilters).toEqual(["in_progress"]);
      expect(merged.priorityFilters).toEqual(["high"]);
      expect(merged.includeNoProject).toBe(true);
      expect(saved).toHaveProperty("projectStatusFilters", projectStatusFilters);
      expect(viewStorePersistOptions("test").partialize(merged)).not.toHaveProperty("projectStatusFilters");
    },
  );

  it("loads a snapshot without a project lifecycle filter without introducing that dimension", () => {
    const current = createStore<IssueViewState>()(viewStoreSlice).getState();
    const merged = mergeViewStatePersisted({ projectFilters: ["project-1"] }, current);
    expect(merged.projectFilters).toEqual(["project-1"]);
    expect(merged).not.toHaveProperty("projectStatusFilters");
  });
});
