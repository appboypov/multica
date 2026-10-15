// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { Squad, SquadMember } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSquads from "../../locales/en/squads.json";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";

const TEST_RESOURCES = { en: { common: enCommon, squads: enSquads } };

// The archived sub-squad is a member of the main squad but, being archived,
// is absent from GET /api/squads; only GET /api/squads/:id returns it.
const mockApi = vi.hoisted(() => ({
  getBaseUrl: vi.fn(() => "http://127.0.0.1:8080"),
  listSquads: vi.fn(),
  getSquad: vi.fn(),
  listSquadMembers: vi.fn(),
  getSquadMemberStatus: vi.fn(),
  listAgents: vi.fn(),
  listMembers: vi.fn(),
  removeSquadMember: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({ api: mockApi }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ id: "ws-1", slug: "acme" }),
  useWorkspacePaths: () => ({
    squads: () => "/acme/squads",
    squadDetail: (id: string) => `/acme/squads/${id}`,
    agentDetail: (id: string) => `/acme/agents/${id}`,
    memberDetail: (id: string) => `/acme/members/${id}`,
    issueDetail: (id: string) => `/acme/issues/${id}`,
    newAgent: () => "/acme/agents/new",
  }),
}));
vi.mock("@multica/core/auth", () => {
  type AuthState = { user: { id: string } | null };
  const state = (): AuthState => ({ user: { id: "user-1" } });
  const useAuthStore = Object.assign(
    (selector?: (s: AuthState) => unknown) =>
      selector ? selector(state()) : state(),
    { getState: state },
  );
  return { useAuthStore };
});
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
// Avatars, the avatar uploader and the instructions editor are irrelevant
// weight for the member rows under test.
vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => <div>actor-avatar</div>,
}));
vi.mock("../../common/avatar-upload-control", () => ({
  AvatarUploadControl: () => null,
}));
vi.mock("../../editor/content-editor", () => ({
  ContentEditor: () => null,
}));

import { SquadDetailPage } from "./squad-detail-page";
import { SquadProfileCard } from "./squad-profile-card";

function makeSquad(overrides: Partial<Squad> = {}): Squad {
  return {
    id: "squad-main",
    workspace_id: "ws-1",
    name: "Main Squad",
    description: "",
    instructions: "",
    avatar_url: null,
    leader_id: "agent-lead",
    creator_id: "user-1",
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

const archivedSubSquad = makeSquad({
  id: "squad-archived-sub",
  name: "Retired Crew",
  archived_at: "2026-10-10T00:00:00Z",
  archived_by: "user-1",
});

const subSquadMember: SquadMember = {
  id: "sm-1",
  squad_id: "squad-main",
  member_type: "squad",
  member_id: archivedSubSquad.id,
  role: "",
  created_at: "2026-10-02T00:00:00Z",
};

const mainSquad = makeSquad({
  member_count: 1,
  member_preview: [
    { member_type: "squad", member_id: archivedSubSquad.id, role: "" },
  ],
});

function renderWithProviders(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const navigation: NavigationAdapter = {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: `/acme/squads/${mainSquad.id}`,
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path) => path,
  };
  return render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <NavigationProvider value={navigation}>
        <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
      </NavigationProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockApi.listSquads.mockResolvedValue([mainSquad]);
  mockApi.getSquad.mockImplementation(async (id: string) =>
    id === archivedSubSquad.id ? archivedSubSquad : mainSquad,
  );
  mockApi.listSquadMembers.mockResolvedValue([subSquadMember]);
  mockApi.getSquadMemberStatus.mockResolvedValue({ members: [] });
  mockApi.listAgents.mockResolvedValue([]);
  mockApi.listMembers.mockResolvedValue([]);
  mockApi.removeSquadMember.mockResolvedValue(undefined);
});

describe("archived sub-squad", () => {
  it("shows its name and the archived pill on the Members tab, and stays removable", async () => {
    renderWithProviders(<SquadDetailPage />);

    expect(await screen.findByText(archivedSubSquad.name)).toBeInTheDocument();
    expect(
      screen.getByText(enSquads.members_tab.status_archived),
    ).toBeInTheDocument();
    expect(mockApi.getSquad).toHaveBeenCalledWith(archivedSubSquad.id);

    fireEvent.click(
      screen.getByRole("button", {
        name: enSquads.members_tab.remove_member_tooltip,
      }),
    );

    await waitFor(() =>
      expect(mockApi.removeSquadMember).toHaveBeenCalledWith(mainSquad.id, {
        member_type: "squad",
        member_id: archivedSubSquad.id,
      }),
    );
  });

  it("shows its name and the archived label in the profile card's members list", async () => {
    renderWithProviders(<SquadProfileCard squadId={mainSquad.id} />);

    expect(await screen.findByText(archivedSubSquad.name)).toBeInTheDocument();
    expect(screen.getByText(enSquads.profile_card.archived)).toBeInTheDocument();
    expect(mockApi.getSquad).toHaveBeenCalledWith(archivedSubSquad.id);
    expect(mockApi.getSquad).not.toHaveBeenCalledWith(mainSquad.id);
  });
});
