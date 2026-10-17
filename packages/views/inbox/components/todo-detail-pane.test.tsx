import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Todo } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { todoFixture } from "../../test/todo-fixture";
import { TodoDetailPane } from "./todo-detail-pane";

const push = vi.hoisted(() => vi.fn());
vi.mock("../../navigation", () => ({ useNavigation: () => ({ push }) }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({ autopilotDetail: (id: string) => `/acme/autopilots/${id}` }),
}));
vi.mock("./inbox-question-pane", () => ({
  InboxQuestionBody: ({ questionId, onOpenIssue }: { questionId: string; onOpenIssue: (id?: string) => void }) => (
    <button type="button" data-testid="question-body" onClick={() => onOpenIssue("q-2")}>{questionId}</button>
  ),
}));
vi.mock("./todo-detail-bodies", () => ({
  TodoMentionBody: () => <div data-testid="mention-body" />,
  TodoIssueBody: ({ todo }: { todo: Todo }) => <div data-testid="issue-body" data-kind={todo.kind} />,
  TodoRunBody: ({ todo }: { todo: Todo }) => <div data-testid="run-body" data-kind={todo.kind} />,
  TodoSystemBody: ({ todo }: { todo: Todo }) => <div data-testid="system-body" data-kind={todo.kind} />,
}));
vi.mock("../../issues/utils/status-label", () => ({ useStatusLabel: () => (status: string) => (status === "todo" ? "Todo" : status) }));

function renderPane(todo: Todo, handlers: { onMarkDone?: () => void; onOpenIssue?: (target: unknown) => void } = {}) {
  return renderWithI18n(
    <TodoDetailPane todo={todo} onMarkDone={handlers.onMarkDone ?? vi.fn()} onOpenIssue={handlers.onOpenIssue ?? vi.fn()} />,
  );
}

describe("TodoDetailPane", () => {
  it("shows the kind, the issue and the line that says when it leaves", () => {
    renderPane(todoFixture());
    expect(screen.getByText("Question for you")).toBeInTheDocument();
    expect(screen.getByText("WORK-412")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "VAT rounding" })).toBeInTheDocument();
    expect(screen.getByText("Leaves your inbox once you answer it.")).toBeInTheDocument();
    expect(screen.getByTestId("question-body")).toHaveTextContent("q-1");
  });

  it("opens the issue at the question from the head and from the card", () => {
    const onOpenIssue = vi.fn();
    renderPane(todoFixture(), { onOpenIssue });
    fireEvent.click(screen.getByRole("button", { name: "Open on WORK-412" }));
    expect(onOpenIssue).toHaveBeenLastCalledWith({ questionId: "q-1" });
    fireEvent.click(screen.getByTestId("question-body"));
    expect(onOpenIssue).toHaveBeenLastCalledWith({ questionId: "q-2" });
  });

  it("shows the card of a question without an issue and opens no issue", () => {
    renderPane(todoFixture({ issue_id: null, issue_identifier: null, issue_title: null, question_id: "q-7", title: "Approve the export" }));
    expect(screen.getByRole("heading", { name: "Approve the export" })).toBeInTheDocument();
    expect(screen.getByTestId("question-body")).toHaveTextContent("q-7");
    expect(screen.queryByTestId("system-body")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Open on/ })).not.toBeInTheDocument();
  });

  it("opens a mention's issue at its comment and names the issue in the leaves line", () => {
    const onOpenIssue = vi.fn();
    renderPane(todoFixture({ kind: "mention", question_id: null, comment_id: "c-9", issue_identifier: "WORK-418" }), { onOpenIssue });
    expect(screen.getByText("Leaves your inbox once you comment on WORK-418.")).toBeInTheDocument();
    expect(screen.getByTestId("mention-body")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open on WORK-418" }));
    expect(onOpenIssue).toHaveBeenCalledWith({ commentId: "c-9" });
  });

  it("marks done from the head", () => {
    const onMarkDone = vi.fn();
    renderPane(todoFixture(), { onMarkDone });
    fireEvent.click(screen.getByRole("button", { name: "Mark done" }));
    expect(onMarkDone).toHaveBeenCalledOnce();
  });

  it.each([
    ["review", "issue-body", "Waiting for your review"],
    ["assigned", "issue-body", "Assigned to you"],
    ["agent_blocked", "run-body", "Agent blocked"],
    ["run_failed", "run-body", "Run failed"],
  ] as const)("shows the %s body", (kind, testId, label) => {
    renderPane(todoFixture({ kind, question_id: null }));
    expect(screen.getByTestId(testId)).toHaveAttribute("data-kind", kind);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("offers Open autopilot for a paused autopilot", () => {
    renderPane(todoFixture({ kind: "autopilot_paused", issue_id: null, issue_identifier: null, issue_title: null, question_id: null, autopilot_id: "ap-1", title: "Daily triage" }));
    expect(screen.getByRole("heading", { name: "Daily triage" })).toBeInTheDocument();
    expect(screen.getByText("Leaves your inbox once the autopilot is resumed.")).toBeInTheDocument();
    expect(screen.getByTestId("system-body")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Open on/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open autopilot" }));
    expect(push).toHaveBeenCalledWith("/acme/autopilots/ap-1");
  });

  it("says an assignment leaves once its issue moves out of its status", () => {
    renderPane(todoFixture({ kind: "assigned", issue_identifier: "WORK-427", issue_status: "todo", question_id: null }));
    expect(screen.getByText("Leaves your inbox once WORK-427 moves out of Todo.")).toBeInTheDocument();
  });
});
