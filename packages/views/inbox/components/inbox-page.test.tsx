import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Todo } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { todoFixture } from "../../test/todo-fixture";
import { InboxPage, nextTodoAfter } from "./inbox-page";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("react-resizable-panels", () => ({
  useDefaultLayout: () => ({ defaultLayout: undefined, onLayoutChanged: vi.fn() }),
}));

const listState: { data: Todo[] | undefined; isPending: boolean; isError: boolean; isFetching: boolean; refetch: () => void } = {
  data: [],
  isPending: false,
  isError: false,
  isFetching: false,
  refetch: vi.fn(),
};
vi.mock("@tanstack/react-query", () => ({ useQuery: () => listState }));

const markDoneMutate = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/todos", () => ({
  todoListOptions: () => ({ queryKey: ["todos", "ws-1", "list"] }),
  useMarkTodoDone: () => ({ mutate: markDoneMutate }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    inbox: () => "/acme/inbox",
    updates: () => "/acme/updates",
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));
vi.mock("@multica/core/modals", () => ({ useModalStore: { getState: () => ({ modal: null }) } }));

const replace = vi.hoisted(() => vi.fn());
const push = vi.hoisted(() => vi.fn());
const nav = vi.hoisted(() => ({ searchParams: new URLSearchParams() }));
vi.mock("../../navigation", () => ({
  useNavigation: () => ({ searchParams: nav.searchParams, replace, push }),
}));
const writeViewState = vi.hoisted(() => vi.fn());
vi.mock("../../platform", () => ({ useViewStateWriter: () => writeViewState }));

const issueDetailProps = vi.hoisted(() => [] as Array<Record<string, unknown>>);
vi.mock("../../issues/components/issue-detail", () => ({
  IssueDetail: (props: Record<string, unknown>) => {
    issueDetailProps.push(props);
    return <div data-testid="issue-detail" />;
  },
  issueHighlightMementoKey: (issueId: string) => `highlight:${issueId}`,
}));

const layout = vi.hoisted(() => ({ compact: false }));
vi.mock("@multica/ui/hooks/use-mobile", () => ({
  useIsMobile: () => layout.compact,
  useIsCompact: () => layout.compact,
}));
vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  ResizablePanel: ({ children, id }: { children: React.ReactNode; id: string }) => <div data-testid={`panel-${id}`}>{children}</div>,
  ResizableHandle: () => null,
}));

vi.mock("./todo-list-item", () => ({
  TodoListItem: ({ todo, isSelected, onClick, onMarkDone }: {
    todo: Todo; isSelected: boolean; onClick: () => void; onMarkDone: () => void;
  }) => (
    <div data-testid={`row-${todo.id}`} data-selected={isSelected ? "true" : "false"}>
      <button type="button" onClick={onClick}>{todo.title}</button>
      <button type="button" onClick={onMarkDone}>{`Mark ${todo.id} done`}</button>
    </div>
  ),
}));
vi.mock("./todo-detail-pane", () => ({
  TodoDetailPane: ({ todo, onMarkDone, onOpenIssue, leadingAction }: {
    todo: Todo; onMarkDone: () => void; onOpenIssue: (target: { questionId?: string }) => void; leadingAction?: React.ReactNode;
  }) => (
    <div data-testid="todo-detail" data-todo-id={todo.id}>
      {leadingAction}
      <button type="button" onClick={onMarkDone}>Detail mark done</button>
      <button type="button" onClick={() => onOpenIssue({ questionId: todo.question_id ?? undefined })}>Detail open issue</button>
    </div>
  ),
}));

const a = todoFixture({ id: "a", title: "Alpha" });
const b = todoFixture({ id: "b", title: "Bravo", question_id: "q-b" });
const c = todoFixture({ id: "c", title: "Charlie" });

function renderPage() {
  return renderWithI18n(<InboxPage />);
}

beforeEach(() => {
  listState.data = [a, b, c];
  listState.isPending = false;
  listState.isError = false;
  listState.isFetching = false;
  nav.searchParams = new URLSearchParams();
  layout.compact = false;
  replace.mockClear();
  push.mockClear();
  markDoneMutate.mockClear();
  writeViewState.mockClear();
  issueDetailProps.length = 0;
});

describe("nextTodoAfter", () => {
  it("prefers the next row, then the previous one, then none", () => {
    expect(nextTodoAfter("b", [a, b, c], [a, c])).toBe("c");
    expect(nextTodoAfter("c", [a, b, c], [a, b])).toBe("b");
    expect(nextTodoAfter("a", [a], [])).toBe("");
  });

  it("skips rows that left together with it", () => {
    expect(nextTodoAfter("a", [a, b, c], [c])).toBe("c");
  });
});

describe("InboxPage", () => {
  it("lists every open to-do with their count", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: "Inbox" })).toBeInTheDocument();
    expect(screen.getByLabelText("3")).toBeInTheDocument();
    expect(screen.getByTestId("row-a")).toBeInTheDocument();
    expect(screen.getByTestId("row-b")).toBeInTheDocument();
    expect(screen.getByTestId("row-c")).toBeInTheDocument();
    expect(screen.getByText("Select a to-do to see what it needs")).toBeInTheDocument();
  });

  it("selects a row by its to-do id and shows its detail", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Bravo" }));
    expect(replace).toHaveBeenLastCalledWith("/acme/inbox?todo=b");
    expect(screen.getByTestId("todo-detail")).toHaveAttribute("data-todo-id", "b");
    expect(screen.getByTestId("row-b")).toHaveAttribute("data-selected", "true");
  });

  it("says nothing waits and offers Updates when no to-do is open", () => {
    listState.data = [];
    renderPage();
    expect(screen.getByText("Nothing waits for you")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open Updates" }));
    expect(push).toHaveBeenCalledWith("/acme/updates");
    expect(screen.queryByLabelText("0")).not.toBeInTheDocument();
  });

  it("Mark done in the detail closes the to-do and selects the next row", () => {
    nav.searchParams = new URLSearchParams("todo=b");
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Detail mark done" }));
    expect(markDoneMutate).toHaveBeenCalledWith("b", expect.anything());
    expect(replace).toHaveBeenLastCalledWith("/acme/inbox?todo=c");
    expect(screen.getByTestId("todo-detail")).toHaveAttribute("data-todo-id", "c");
  });

  it("Mark done on the last row selects the one above it", () => {
    nav.searchParams = new URLSearchParams("todo=c");
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Mark c done" }));
    expect(markDoneMutate).toHaveBeenCalledWith("c", expect.anything());
    expect(replace).toHaveBeenLastCalledWith("/acme/inbox?todo=b");
  });

  it("Mark done on another row keeps the selection", () => {
    nav.searchParams = new URLSearchParams("todo=b");
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Mark a done" }));
    expect(markDoneMutate).toHaveBeenCalledWith("a", expect.anything());
    expect(replace).not.toHaveBeenCalled();
    expect(screen.getByTestId("todo-detail")).toHaveAttribute("data-todo-id", "b");
  });

  it("selects the next row once the selected to-do leaves by itself", () => {
    nav.searchParams = new URLSearchParams("todo=a");
    const { rerender } = renderPage();
    expect(screen.getByTestId("todo-detail")).toHaveAttribute("data-todo-id", "a");
    listState.data = [b, c];
    rerender(<InboxPage />);
    expect(replace).toHaveBeenLastCalledWith("/acme/inbox?todo=b");
    expect(screen.getByTestId("todo-detail")).toHaveAttribute("data-todo-id", "b");
  });

  it("clears a link to a to-do that is not open, once the list has loaded", () => {
    nav.searchParams = new URLSearchParams("todo=gone");
    listState.isFetching = true;
    const { rerender } = renderPage();
    expect(replace).not.toHaveBeenCalled();
    listState.isFetching = false;
    rerender(<InboxPage />);
    expect(replace).toHaveBeenLastCalledWith("/acme/inbox");
    expect(screen.queryByTestId("todo-detail")).not.toBeInTheDocument();
  });

  it("opens the issue at the question and returns to the to-do on a row click", () => {
    nav.searchParams = new URLSearchParams("todo=b");
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Detail open issue" }));
    expect(writeViewState).toHaveBeenCalledWith("highlight:issue-412", undefined);
    expect(screen.getByTestId("issue-detail")).toBeInTheDocument();
    expect(issueDetailProps.at(-1)).toMatchObject({ issueId: "issue-412", highlightQuestionId: "q-b", leadingAction: false });
    fireEvent.click(screen.getByRole("button", { name: "Bravo" }));
    expect(screen.queryByTestId("issue-detail")).not.toBeInTheDocument();
    expect(screen.getByTestId("todo-detail")).toHaveAttribute("data-todo-id", "b");
  });

  it("shows only the detail on a phone, with a way back to the list", () => {
    layout.compact = true;
    nav.searchParams = new URLSearchParams("todo=a");
    renderPage();
    expect(screen.queryByTestId("row-a")).not.toBeInTheDocument();
    expect(screen.getByTestId("todo-detail")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Inbox" }));
    expect(replace).toHaveBeenLastCalledWith("/acme/inbox");
    expect(screen.getByTestId("row-a")).toBeInTheDocument();
  });

  it("offers a retry when the list fails to load", () => {
    listState.data = undefined;
    listState.isError = true;
    renderPage();
    expect(screen.getByText("Could not load your to-dos")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(listState.refetch).toHaveBeenCalled();
  });
});
