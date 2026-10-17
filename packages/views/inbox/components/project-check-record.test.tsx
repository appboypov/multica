import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { ProjectCheckRecord } from "./project-check-record";

const state = vi.hoisted(() => ({ mode: vi.fn(), recipient: true, kind: "feature", sampling: true, historyError: false }));
vi.mock("@multica/core/todos", () => ({
  useProjectCheckTrust: () => ({ data: [{ project_id: "p", kind: state.kind, passed: 3, feedback: 1, automatic_passed: 4, sampling: state.sampling, can_manage: state.recipient }], isPending: false, isError: false }),
  useSetCheckSampling: () => ({ mutate: state.mode, isPending: false, isError: false }),
  useCheckHistory: () => ({ data: [{ id: "c", title: "Invoice totals", answer: "feedback", note: "Round totals", link: "https://example.test/work", sampled: true, automatic: false, answered_at: "2026-10-17T10:00:00Z" }], isPending: false, isError: state.historyError, refetch: vi.fn() }),
}));
beforeEach(() => { vi.clearAllMocks(); state.recipient = true; state.kind = "feature"; state.sampling = true; state.historyError = false; });
describe("ProjectCheckRecord", () => {
  it("shows counts and history without switching after feedback", () => {
    renderWithI18n(<ProjectCheckRecord projectId="p" />);
    expect(screen.getByText("3 passed, 1 with feedback, 4 automatic passes.")).toBeInTheDocument();
    expect(state.mode).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Show check history" }));
    expect(screen.getByText("Invoice totals")).toBeInTheDocument();
    expect(screen.getByText("Round totals")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Check every piece" }));
    expect(state.mode).toHaveBeenCalledWith({ projectId: "p", kind: "feature", sampling: false });
  });
  it("offers no mode control to another member", () => {
    state.recipient = false;
    renderWithI18n(<ProjectCheckRecord projectId="p" />);
    expect(screen.queryByRole("button", { name: "Check every piece" })).not.toBeInTheDocument();
  });
  it("never offers release sampling", () => {
    state.kind = "release"; state.sampling = false;
    renderWithI18n(<ProjectCheckRecord projectId="p" />);
    expect(screen.queryByRole("button", { name: "Use samples" })).not.toBeInTheDocument();
  });
});
