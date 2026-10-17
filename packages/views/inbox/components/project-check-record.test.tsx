import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import type { CheckHistoryItem, CheckTrust } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ProjectCheckRecord } from "./project-check-record";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

const feature: CheckTrust = { project_id: "p", kind: "feature", member_id: "brian", passed: 3, feedback: 1, automatic_passed: 4, sampling: true, sample_opens: 5, can_manage: true };
const answered: CheckHistoryItem = { id: "c", title: "Invoice totals", project_id: "p", check_kind: "feature", answer: "feedback", note: "Round totals", link: "https://example.test/work", sampled: true, automatic: false, answered_at: "2026-10-17T10:00:00Z" };
const api = { getProjectCheckTrust: vi.fn(), setCheckSampling: vi.fn(), getCheckHistory: vi.fn() };

beforeEach(() => {
  vi.clearAllMocks();
  api.getProjectCheckTrust.mockResolvedValue([feature]);
  api.setCheckSampling.mockResolvedValue({ ...feature, sampling: false });
  api.getCheckHistory.mockResolvedValue([answered]);
  setApiInstance(api as unknown as ApiClient);
});

function renderRecord() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return renderWithI18n(<QueryClientProvider client={client}><ProjectCheckRecord projectId="p" /></QueryClientProvider>);
}

describe("ProjectCheckRecord", () => {
  it("shows counts and history without switching after feedback", async () => {
    renderRecord();
    expect(await screen.findByText("3 passed, 1 with feedback, 4 automatic passes.")).toBeInTheDocument();
    expect(api.setCheckSampling).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Show check history" }));
    expect(await screen.findByText("Invoice totals")).toBeInTheDocument();
    expect(screen.getByText("Round totals")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Check every piece" }));
    await waitFor(() => expect(api.setCheckSampling).toHaveBeenCalledWith("p", "feature", false));
  });
  it("keeps the shown mode and shows the error when the mode change fails", async () => {
    api.setCheckSampling.mockRejectedValue(new Error("server unavailable"));
    renderRecord();
    fireEvent.click(await screen.findByRole("button", { name: "Check every piece" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not save the check mode.");
    expect(screen.getByText("Samples")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Check every piece" })).toBeEnabled();
  });
  it("offers no mode control to another member", async () => {
    api.getProjectCheckTrust.mockResolvedValue([{ ...feature, can_manage: false }]);
    renderRecord();
    await screen.findByText("3 passed, 1 with feedback, 4 automatic passes.");
    expect(screen.queryByRole("button", { name: "Check every piece" })).not.toBeInTheDocument();
  });
  it("never offers release sampling", async () => {
    api.getProjectCheckTrust.mockResolvedValue([{ ...feature, kind: "release", sampling: false }]);
    renderRecord();
    await screen.findByText("3 passed, 1 with feedback, 4 automatic passes.");
    expect(screen.queryByRole("button", { name: "Use samples" })).not.toBeInTheDocument();
  });
});
