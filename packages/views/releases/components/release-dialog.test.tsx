import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type * as CoreAPI from "@multica/core/api";
import type * as CorePermissions from "@multica/core/permissions";
import type { Agent, Release, Repository, Squad } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ReleaseDialog } from "./release-dialog";

const api = vi.hoisted(() => ({
  listReleases: vi.fn(),
  listRepositories: vi.fn(),
  planRelease: vi.fn(),
  editRelease: vi.fn(),
  listAgents: vi.fn(),
  listSquads: vi.fn(),
}));
const NAMES: Record<string, string> = { "u-brian": "Brian", "agent-pieter": "Pieter", "squad-1": "Release squad" };

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/api", async (importOriginal) => ({ ...await importOriginal<typeof CoreAPI>(), api }));
vi.mock("@multica/core/permissions", async (importOriginal) => ({
  ...await importOriginal<typeof CorePermissions>(),
  useCurrentMember: () => ({ userId: "u-brian", role: "member", member: null, isLoading: false }),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: (_type: string, id: string) => NAMES[id] ?? "Unknown" }),
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span data-testid="avatar" /> }));

const repository = {
  id: "repo-1",
  full_name: "appboypov/rebl",
  last_tag: "0.9.0",
  default_branch: "main",
  version_policy: "tags",
} as unknown as Repository;

const base = (overrides: Partial<Release> & Pick<Release, "id" | "version" | "title" | "position">): Release => ({
  repository_id: "repo-1",
  description: "",
  bump: "minor",
  shipped_at: null,
  created_at: "",
  updated_at: "",
  milestones: [],
  owner_type: null,
  owner_id: null,
  owner_name: null,
  owner_avatar_url: null,
  ...overrides,
});
const shipped = base({
  id: "r-100",
  position: 0,
  version: "1.0.0",
  title: "First production release",
  shipped_at: "2026-01-10T00:00:00Z",
  owner_type: "agent",
  owner_id: "agent-pieter",
  owner_name: "Pieter",
});
const planned = base({
  id: "r-110",
  position: 1,
  version: "1.1.0",
  title: "Account and profile",
  owner_type: "agent",
  owner_id: "agent-pieter",
  owner_name: "Pieter",
});

const agents = [
  { id: "agent-pieter", name: "Pieter", owner_id: "u-brian", permission_mode: "private", visibility: "workspace", archived_at: null },
] as unknown as Agent[];
const squads = [{ id: "squad-1", name: "Release squad", archived_at: null }] as unknown as Squad[];

function mount(data: Record<string, unknown>) {
  api.listReleases.mockResolvedValue({ releases: [shipped, planned] });
  api.listRepositories.mockResolvedValue({ repositories: [repository] });
  api.listAgents.mockResolvedValue(agents);
  api.listSquads.mockResolvedValue(squads);
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const onClose = vi.fn();
  renderWithI18n(
    <QueryClientProvider client={qc}>
      <ReleaseDialog onClose={onClose} data={{ repositoryId: "repo-1", ...data }} />
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => vi.resetAllMocks());

describe("release dialog owner", () => {
  it("plans with No owner by default and sends the picked squad", async () => {
    const user = userEvent.setup();
    api.planRelease.mockResolvedValue({ releases: [shipped, planned] });
    const onClose = mount({});

    await user.type(await screen.findByLabelText("Title"), "Push notifications");
    await user.click(screen.getByRole("button", { name: "Owner No owner" }));
    expect(await screen.findByRole("button", { name: "Pieter" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Brian" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Release squad" }));
    await user.click(screen.getByRole("button", { name: /^Plan v/ }));

    await waitFor(() => expect(api.planRelease).toHaveBeenCalled());
    expect(api.planRelease.mock.calls[0]?.[1]).toMatchObject({
      title: "Push notifications",
      owner_type: "squad",
      owner_id: "squad-1",
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("keeps an unchanged owner out of an edit so the server keeps it", async () => {
    const user = userEvent.setup();
    api.editRelease.mockResolvedValue({ releases: [shipped, planned] });
    mount({ releaseId: "r-110" });

    expect(await screen.findByRole("button", { name: "Owner Pieter" })).toBeTruthy();
    const title = screen.getByLabelText("Title");
    await user.clear(title);
    await user.type(title, "Accounts");
    await user.click(screen.getByRole("button", { name: "Save release" }));

    await waitFor(() => expect(api.editRelease).toHaveBeenCalled());
    const body = api.editRelease.mock.calls[0]?.[1];
    expect(body).toMatchObject({ title: "Accounts" });
    expect(body).not.toHaveProperty("owner_type");
    expect(body).not.toHaveProperty("owner_id");
  });

  it("removes the owner of an unshipped release on save", async () => {
    const user = userEvent.setup();
    api.editRelease.mockResolvedValue({ releases: [shipped, { ...planned, owner_type: null, owner_id: null, owner_name: null }] });
    mount({ releaseId: "r-110" });

    await user.click(await screen.findByRole("button", { name: "Owner Pieter" }));
    await user.click(await screen.findByRole("button", { name: "No owner" }));
    expect(await screen.findByRole("button", { name: "Owner No owner" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Save release" }));

    await waitFor(() =>
      expect(api.editRelease).toHaveBeenCalledWith("r-110", expect.objectContaining({ owner_type: null, owner_id: null })),
    );
  });

  it("shows a shipped release's owner read-only", async () => {
    mount({ releaseId: "r-100" });
    expect(await screen.findByText("Pieter")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Owner/ })).toBeNull();
  });
});
