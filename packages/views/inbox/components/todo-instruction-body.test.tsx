import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { todoFixture } from "../../test/todo-fixture";
import { TodoInstructionBody } from "./todo-instruction-body";

const mutate = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/todos", () => ({ useAnswerCheckTodo: () => ({ mutate, isPending: false, isError: false }) }));
const details = { check_id: "check-1", file: "skills/totals.md", link: "https://example.test/commit", note: "Round totals before display", undo_requested: false };
beforeEach(() => vi.clearAllMocks());
describe("TodoInstructionBody", () => {
  it("shows the change and lets Brian request Undo or Keep", () => {
    renderWithI18n(<TodoInstructionBody todo={todoFixture({ kind: "instruction_change", details })} />);
    expect(screen.getByText("skills/totals.md")).toBeInTheDocument();
    expect(screen.getByText("For check check-1")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open instruction commit" })).toHaveAttribute("href", details.link);
    fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(mutate).toHaveBeenCalledWith({ id: "todo-1", answer: "undo", note: "" });
    fireEvent.click(screen.getByRole("button", { name: "Keep" }));
    expect(mutate).toHaveBeenCalledWith({ id: "todo-1", answer: "keep", note: "" });
  });
  it("waits for the agent without claiming an undo", () => {
    renderWithI18n(<TodoInstructionBody todo={todoFixture({ kind: "instruction_change", details: { ...details, undo_requested: true } })} />);
    expect(screen.getByText("Undo requested. Waiting for the agent to report the result.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Undo" })).toBeDisabled();
    expect(screen.queryByRole("link", { name: "Open undo commit" })).not.toBeInTheDocument();
  });
  it("shows the reported undo and rejects unsafe links", () => {
    renderWithI18n(<TodoInstructionBody todo={todoFixture({ kind: "instruction_change", details: { ...details, link: "javascript:alert(1)", undo_requested: true, undo_note: "Reverted the instruction", undo_link: "https://example.test/revert" } })} />);
    expect(screen.getByText("Reverted the instruction")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Open instruction commit" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open undo commit" })).toHaveAttribute("href", "https://example.test/revert");
  });
});
