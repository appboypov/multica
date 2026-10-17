import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { todoFixture } from "../../test/todo-fixture";
import { TodoCheckBody } from "./todo-check-body";

const state = vi.hoisted(() => ({ mutate: vi.fn(), setMode: vi.fn(), pending: false, error: false, trustError: false, refetch: vi.fn() }));
vi.mock("@multica/core/todos", () => ({
  useAnswerCheckTodo: () => ({ mutate: state.mutate, isPending: state.pending, isError: state.error }),
  useProjectCheckTrust: () => ({ isPending: false, isError: state.trustError, refetch: state.refetch, data: [{ project_id: "project-1", kind: "feature", sampling: true, can_manage: true, passed: 5, feedback: 0, automatic_passed: 4 }] }),
  useSetCheckSampling: () => ({ mutate: state.setMode, isPending: false, isError: false }),
  useCheckHistory: () => ({ isPending: false, isError: false, data: [], refetch: vi.fn() }),
}));
const details = { project_id: "project-1", check_kind: "feature", link: "https://staging.example.test", sampled: true };
beforeEach(() => { vi.clearAllMocks(); state.pending = false; state.error = false; state.trustError = false; });

describe("TodoCheckBody", () => {
  it("shows the kind, staging link, sample and feedback action", () => {
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details })} />);
    expect(screen.getAllByRole("heading", { name: "Feature" })).toHaveLength(2);
    expect(screen.getByRole("link", { name: "Open work" })).toHaveAttribute("href", details.link + "/");
    expect(screen.getByText(/This piece is a sample/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send feedback" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Feedback"), { target: { value: "Round totals" } });
    fireEvent.click(screen.getByRole("button", { name: "Send feedback" }));
    expect(state.mutate).toHaveBeenCalledWith({ id: "todo-1", answer: "feedback", note: "Round totals" });
  });
  it("changes mode only when Brian asks", () => {
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details })} />);
    expect(state.setMode).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Check every piece" }));
    expect(state.setMode).toHaveBeenCalledWith({ projectId: "project-1", kind: "feature", sampling: false });
    expect(state.mutate).not.toHaveBeenCalled();
  });
  it("keeps actions disabled while saving and shows a retryable error", () => {
    state.pending = true; state.error = true;
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details })} />);
    expect(screen.getByRole("button", { name: "Pass" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Could not save your answer");
  });
  it("keeps Pass and Feedback on for a kind this app does not know", () => {
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details: { ...details, check_kind: "security_review" } })} />);
    expect(screen.getByRole("heading", { name: "Check for you" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Pass" }));
    expect(state.mutate).toHaveBeenCalledWith({ id: "todo-1", answer: "pass", note: "" });
    fireEvent.change(screen.getByLabelText("Feedback"), { target: { value: "Missing audit log" } });
    fireEvent.click(screen.getByRole("button", { name: "Send feedback" }));
    expect(state.mutate).toHaveBeenLastCalledWith({ id: "todo-1", answer: "feedback", note: "Missing audit log" });
  });
  it("shows project trust and protects work links", () => {
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details: { ...details, link: "javascript:alert(1)" } })} />);
    expect(screen.queryByRole("link", { name: "Open work" })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Project check record" })).toHaveTextContent("Samples");
    expect(screen.getByText("5 passed, 0 with feedback, 4 automatic passes.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show check history" }));
    expect(screen.getByText("No answered checks yet.")).toBeInTheDocument();
  });
});
