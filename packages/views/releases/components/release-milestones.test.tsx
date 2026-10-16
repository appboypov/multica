import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { releaseKeys, releaseListOptions } from "@multica/core/releases";
import type { Release } from "@multica/core/types";
import type * as CoreAPI from "@multica/core/api";
import { renderWithI18n } from "../../test/i18n";
import { ReleaseMilestones } from "./release-milestones";

const mocks = vi.hoisted(() => ({ add: vi.fn(), edit: vi.fn(), remove: vi.fn(), list: vi.fn() }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof CoreAPI>(),
  api: {
    addReleaseMilestone: mocks.add, editReleaseMilestone: mocks.edit,
    removeReleaseMilestone: mocks.remove, listReleases: mocks.list,
  },
  clientErrorMessage: (err: unknown) => err instanceof Error ? err.message : undefined,
}));

const milestone = { id: "m-1", release_id: "r-1", name: "UI prototype", date: "2026-11-02", created_at: "", updated_at: "" };
const release: Release = { id: "r-1", repository_id: "repo-1", position: 0, version: "1.0.0", title: "First", description: "", bump: null, shipped_at: null, created_at: "", updated_at: "", milestones: [milestone], owner_type: null, owner_id: null, owner_name: null, owner_avatar_url: null };

function mount(initial: Release = release) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  qc.setQueryData(releaseKeys.list("ws-1"), { releases: [initial] });
  mocks.list.mockResolvedValue({ releases: [initial] });
  function Harness() {
    const { data } = useQuery(releaseListOptions("ws-1"));
    return data?.[0] ? <ReleaseMilestones release={data[0]} /> : null;
  }
  renderWithI18n(<QueryClientProvider client={qc}><Harness /></QueryClientProvider>);
  return qc;
}

beforeEach(() => vi.resetAllMocks());

describe("release milestones", () => {
  it("shows milestone names and calendar days without timezone conversion", () => {
    mount();
    expect(screen.getByText("UI prototype")).toBeTruthy();
    expect(screen.getByText("2026-11-02").getAttribute("datetime")).toBe("2026-11-02");
  });

  it("adds a milestone and settles the release query cache", async () => {
    const user = userEvent.setup();
    const qc = mount({ ...release, milestones: [] });
    mocks.add.mockResolvedValue({ releases: [release] });
    await user.click(screen.getByRole("button", { name: "Add milestone" }));
    await user.type(screen.getByLabelText("Name"), "UI prototype");
    await user.type(screen.getByLabelText("Date"), "2026-11-02");
    await user.click(screen.getByRole("button", { name: "Save milestone" }));
    await waitFor(() => expect(mocks.add).toHaveBeenCalledWith("r-1", { name: "UI prototype", date: "2026-11-02" }));
    expect(await screen.findByText("UI prototype")).toBeTruthy();
    expect(qc.getQueryData(releaseKeys.list("ws-1"))).toEqual({ releases: [release] });
  });

  it("edits a milestone and requires confirmation before deleting it", async () => {
    const user = userEvent.setup();
    mount();
    mocks.edit.mockResolvedValue({ releases: [{ ...release, milestones: [{ ...milestone, name: "Beta acceptance" }] }] });
    await user.click(screen.getByRole("button", { name: "Edit UI prototype" }));
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Beta acceptance");
    await user.click(screen.getByRole("button", { name: "Save milestone" }));
    expect(await screen.findByText("Beta acceptance")).toBeTruthy();
    expect(mocks.edit).toHaveBeenCalledWith("m-1", { name: "Beta acceptance", date: "2026-11-02" });
    mocks.remove.mockResolvedValue({ releases: [{ ...release, milestones: [] }] });
    await user.click(screen.getByRole("button", { name: "Remove Beta acceptance" }));
    expect(mocks.remove).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Remove milestone" }));
    await waitFor(() => expect(mocks.remove).toHaveBeenCalledWith("m-1"));
    await waitFor(() => expect(screen.queryByText("Beta acceptance")).toBeNull());
  });

  it("preserves inputs and reports failure without clearing a milestone", async () => {
    const user = userEvent.setup();
    mount();
    mocks.edit.mockRejectedValue(new Error("Could not save. Try again."));
    await user.click(screen.getByRole("button", { name: "Edit UI prototype" }));
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Review");
    await user.click(screen.getByRole("button", { name: "Save milestone" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not save. Try again.");
    expect(screen.getByLabelText("Name")).toHaveValue("Review");
    expect(screen.getByText("UI prototype")).toBeTruthy();
  });
});
