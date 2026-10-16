import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { todoFixture } from "../../test/todo-fixture";
import { TodoCheckBody } from "./todo-check-body";

const state = vi.hoisted(() => ({ mutate: vi.fn(), pending: false, error: false, trustError: false, refetch: vi.fn() }));
vi.mock("@multica/core/todos", () => ({
  useAnswerCheckTodo: () => ({ mutate: state.mutate, isPending: state.pending, isError: state.error }),
  useProjectCheckTrust: () => ({ isPending: false, isError: state.trustError, refetch: state.refetch, data: [{ kind: "feature", automatic: true, passed: 5, feedback: 0, automatic_passed: 4 }] }),
}));
const details = { project_id: "project-1", check_kind: "feature", link: "https://staging.example.test", sampled: true };
beforeEach(() => { vi.clearAllMocks(); state.pending = false; state.error = false; state.trustError = false; });

describe("TodoCheckBody", () => {
  it("shows the kind, staging link, sample and feedback action", () => {
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details })} />);
    expect(screen.getByRole("heading", { name: "Feature" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open work" })).toHaveAttribute("href", details.link + "/");
    expect(screen.getByText(/This is a sample/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send feedback" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Feedback"), { target: { value: "Round totals" } });
    fireEvent.click(screen.getByRole("button", { name: "Send feedback" }));
    expect(state.mutate).toHaveBeenCalledWith({ id: "todo-1", answer: "feedback", note: "Round totals" });
  });
  it("shows proposal evidence and only agrees on Yes", () => {
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "trust_proposal", details: { ...details, passed: 5, feedback: 1, streak: 5 } })} />);
    expect(screen.getByText("5 passed, 1 with feedback. Clean streak: 5.")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Yes, make automatic" }));
    expect(state.mutate).toHaveBeenCalledWith({ id: "todo-1", answer: "yes", note: "" });
  });
  it("keeps actions disabled while saving and shows a retryable error", () => {
    state.pending = true; state.error = true;
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details })} />);
    expect(screen.getByRole("button", { name: "Pass" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Could not save your answer");
  });
  it("shows project trust and protects work links", () => {
    renderWithI18n(<TodoCheckBody todo={todoFixture({ kind: "check", details: { ...details, link: "javascript:alert(1)" } })} />);
    expect(screen.queryByRole("link", { name: "Open work" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Project check trust"));
    expect(screen.getByRole("table")).toHaveTextContent("Automatic");
    expect(screen.getByRole("table")).toHaveTextContent("Human passes");
  });
});
