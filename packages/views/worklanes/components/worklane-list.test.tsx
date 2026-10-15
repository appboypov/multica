import { type ReactNode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { WorklaneIssueRows } from "./worklane-list";

vi.mock("@dnd-kit/core", () => ({
  DndContext: ({ children, onDragEnd }: { children: ReactNode; onDragEnd: (event: { active: { id: string }; over: { id: string } | null }) => void }) => <div>
    {children}
    <button onClick={() => onDragEnd({ active: { id: "b" }, over: { id: "a" } })}>Move up</button>
    <button onClick={() => onDragEnd({ active: { id: "a" }, over: { id: "b" } })}>Move down</button>
    <button onClick={() => onDragEnd({ active: { id: "b" }, over: null })}>Cancel</button>
    <button onClick={() => onDragEnd({ active: { id: "b" }, over: { id: "outside" } })}>Outside</button>
  </div>,
  PointerSensor: class {}, KeyboardSensor: class {}, closestCenter: vi.fn(),
  useSensor: vi.fn(), useSensors: vi.fn(),
}));
vi.mock("@dnd-kit/sortable", () => ({ SortableContext: ({ children }: { children: ReactNode }) => children, sortableKeyboardCoordinates: vi.fn(), verticalListSortingStrategy: {} }));
vi.mock("../../issues/components/list-row", () => ({
  ListRow: ({ issue }: { issue: Issue }) => <div>{issue.title}</div>,
  DraggableListRow: ({ issue }: { issue: Issue }) => <div data-testid="draggable">{issue.title}</div>,
}));
vi.mock("../../i18n", () => ({ useT: () => ({ t: () => "Failed to move issue" }) }));

const issues: Issue[] = ["a", "b"].map((id) => ({
  id, workspace_id: "ws", number: 1, identifier: id, title: id, description: null,
  status: "todo", priority: "none", assignee_type: null, assignee_id: null,
  creator_type: "member", creator_id: "user", parent_issue_id: null, project_id: null,
  worklane_id: "lane", release_id: null, position: 0, stage: null, start_date: null,
  due_date: null, metadata: {}, properties: {}, created_at: "", updated_at: "",
}));

describe("worklane list drag", () => {
  it("sends relative moves in both directions and ignores cancelled or foreign drops", () => {
    const mutate = vi.fn();
    render(<WorklaneIssueRows issues={issues} move={{ isPending: false, mutate }} />);
    fireEvent.click(screen.getByText("Move up"));
    expect(mutate).toHaveBeenLastCalledWith({ issueId: "b", before: "a" }, expect.any(Object));
    fireEvent.click(screen.getByText("Move down"));
    expect(mutate).toHaveBeenLastCalledWith({ issueId: "a", after: "b" }, expect.any(Object));
    fireEvent.click(screen.getByText("Cancel"));
    fireEvent.click(screen.getByText("Outside"));
    expect(mutate).toHaveBeenCalledTimes(2);
  });

  it("blocks a second drag while the canonical order is pending", () => {
    const mutate = vi.fn();
    render(<WorklaneIssueRows issues={issues} move={{ isPending: true, mutate }} />);
    fireEvent.click(screen.getByText("Move up"));
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.queryByTestId("draggable")).toBeNull();
  });
});
