import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { buildIssueStatusCatalog } from "@multica/core/issue-statuses";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import type { Todo } from "@multica/core/types";
import { todoFixture as todo } from "../../test/todo-fixture";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { TodoListItem } from "./todo-list-item";

vi.mock("@multica/core/issue-statuses/hooks", () => ({
  useIssueStatuses: () => buildIssueStatusCatalog(undefined),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: (type: string) => (type === "agent" ? "Pieter" : "Anouk") }),
}));
vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: ({ actorType, actorId }: { actorType: string; actorId: string }) => (
    <span data-testid="actor-avatar" data-actor-type={actorType} data-actor-id={actorId} />
  ),
}));

function adapter(): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/inbox",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (p) => p,
  };
}

function renderRow(row: Todo, handlers: { onClick?: () => void; onMarkDone?: () => void } = {}) {
  return renderWithI18n(
    <WorkspaceSlugProvider slug="acme">
      <NavigationProvider value={adapter()}>
        <TodoListItem
          todo={row}
          isSelected={false}
          onClick={handlers.onClick ?? vi.fn()}
          onMarkDone={handlers.onMarkDone ?? vi.fn()}
        />
      </NavigationProvider>
    </WorkspaceSlugProvider>,
  );
}

describe("TodoListItem", () => {
  it("shows the issue title, the kind and who it waits on", () => {
    renderRow(todo());
    expect(screen.getByText("VAT rounding")).toBeInTheDocument();
    expect(screen.getByText(/Question for you/)).toBeInTheDocument();
    expect(screen.getByText(/Pieter/)).toBeInTheDocument();
    expect(screen.getByTestId("actor-avatar")).toHaveAttribute("data-actor-id", "agent-1");
  });

  it("labels an approval question as an approval", () => {
    renderRow(todo({ question_answer_type: "approval" }));
    expect(screen.getByText(/Approval for you/)).toBeInTheDocument();
  });

  it("shows Overdue instead of the age once past its due date", () => {
    renderRow(todo({ overdue: true }));
    expect(screen.getByText("Overdue")).toBeInTheDocument();
    expect(screen.queryByText("Just now")).not.toBeInTheDocument();
  });

  it("shows a settings mark, not an avatar, for a system problem", () => {
    renderRow(todo({ kind: "autopilot_paused", issue_id: null, actor_type: null, actor_id: null, title: "Daily triage" }));
    expect(screen.queryByTestId("actor-avatar")).not.toBeInTheDocument();
    expect(screen.getByText("Daily triage")).toBeInTheDocument();
    expect(screen.getByText(/Autopilot paused/)).toBeInTheDocument();
  });

  it("marks done from the row without selecting it", () => {
    const onClick = vi.fn();
    const onMarkDone = vi.fn();
    renderRow(todo(), { onClick, onMarkDone });
    fireEvent.click(screen.getByRole("button", { name: "Mark done" }));
    expect(onMarkDone).toHaveBeenCalledOnce();
    expect(onClick).not.toHaveBeenCalled();
  });

  it("selects on click and on Enter", () => {
    const onClick = vi.fn();
    const { container } = renderRow(todo(), { onClick });
    const row = container.querySelector("[data-todo-id='todo-1']")!;
    fireEvent.click(row);
    fireEvent.keyDown(row, { key: "Enter" });
    expect(onClick).toHaveBeenCalledTimes(2);
  });
});
