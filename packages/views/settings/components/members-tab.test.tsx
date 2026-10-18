import { useSyncExternalStore, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { configStore } from "@multica/core/config";
import { BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG } from "@multica/core/feature-flags";
import { renderWithI18n } from "../../test/i18n";

const mockCreateMember = vi.hoisted(() => vi.fn());
const mockUpdateMember = vi.hoisted(() => vi.fn());
// The members refresh after a write; a test holds it pending to model a slow refetch.
const mockInvalidateQueries = vi.hoisted(() => vi.fn(() => Promise.resolve()));
// The URL the settings router holds; `setSearch` stands in for navigation.
const url = vi.hoisted(() => {
  const listeners = new Set<() => void>();
  const state = { search: "" };
  return {
    get: () => state.search,
    set: (search: string) => {
      state.search = search;
      listeners.forEach((listener) => listener());
    },
    subscribe: (listener: () => void) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
});
const data = vi.hoisted(() => ({
  members: [] as Array<Record<string, unknown>>,
  invitations: [] as Array<Record<string, unknown>>,
  shareLinks: [] as Array<Record<string, unknown>>,
  summary: null as Record<string, unknown> | null,
  organisations: [] as Array<Record<string, unknown>>,
}));

vi.mock("@tanstack/react-query", () => ({
  queryOptions: <T,>(opts: T) => opts,
  useQuery: (opts: { queryKey: unknown[] }) => {
    const key = JSON.stringify(opts.queryKey);
    if (key.includes("invitations")) return { data: data.invitations };
    if (key.includes("share-links")) return { data: data.shareLinks };
    if (key.includes("organisations")) return { data: data.organisations };
    if (key.includes("members")) return { data: data.members };
    return { data: data.summary };
  },
  useQueryClient: () => ({
    invalidateQueries: mockInvalidateQueries,
    fetchQuery: vi.fn(),
  }),
}));
vi.mock("@multica/core/billing", () => ({
  usePreviewWorkspaceSeatPurchase: () => ({ mutateAsync: vi.fn() }),
  usePurchaseWorkspaceSeats: () => ({ mutateAsync: vi.fn() }),
  workspaceSubscriptionSummaryOptions: (wsId: string) => ({
    queryKey: ["billing", wsId, "summary"],
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ id: "ws-1", name: "Acme", slug: "acme" }),
}));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: { createMember: mockCreateMember, updateMember: mockUpdateMember },
}));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  const useAuthStore = Object.assign(
    (selector?: (s: typeof state) => unknown) => (selector ? selector(state) : state),
    { getState: () => state },
  );
  return { useAuthStore };
});
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));
vi.mock("../../navigation", () => ({
  AppLink: ({ href, children, ...rest }: { href: string; children: ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
  useOptionalNavigation: () => {
    const search = useSyncExternalStore(url.subscribe, url.get);
    const go = (href: string) => url.set(href.split("?")[1] ?? "");
    return {
      pathname: "/acme/settings",
      searchParams: new URLSearchParams(search),
      push: go,
      replace: go,
    };
  },
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { MembersTab } from "./members-tab";

const member = (id: string, name: string, role: string, organisationIds: string[] = []) => ({
  id: `m-${id}`,
  user_id: id,
  workspace_id: "ws-1",
  role,
  name,
  email: `${name.toLowerCase().split(" ")[0]}@acme.dev`,
  avatar_url: null,
  organisation_ids: organisationIds,
  created_at: "2025-01-08T00:00:00Z",
});

beforeEach(() => {
  vi.clearAllMocks();
  url.set("tab=members");
  data.members = [
    member("user-1", "Ada Lovelace", "owner"),
    member("user-2", "Grace Hopper", "admin"),
    member("user-3", "Alan Turing", "member"),
  ];
  data.invitations = [
    {
      id: "inv-1",
      workspace_id: "ws-1",
      inviter_id: "user-1",
      invitee_email: "linus@acme.dev",
      invitee_user_id: null,
      role: "member",
      organisation_ids: [],
      status: "pending",
      created_at: "2025-03-01T00:00:00Z",
      updated_at: "2025-03-01T00:00:00Z",
      expires_at: "2025-03-08T00:00:00Z",
    },
  ];
  data.shareLinks = [];
  data.summary = null;
  data.organisations = [
    { id: "org-a", name: "Northwind" },
    { id: "org-b", name: "Globex" },
  ];
  configStore.getState().setFeatureFlags({});
});

describe("MembersTab", () => {
  it("lists members with their role and marks the current user", () => {
    renderWithI18n(<MembersTab />);

    expect(screen.getByRole("tab", { name: /Members\s*3/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("You")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Actions for Grace Hopper" })).toBeInTheDocument();
    // Nobody manages their own membership from this list.
    expect(screen.queryByRole("button", { name: "Actions for Ada Lovelace" })).toBeNull();
  });

  it("filters members by name or email", () => {
    renderWithI18n(<MembersTab />);
    const search = screen.getByRole("searchbox", { name: "Search name or email" });

    fireEvent.change(search, { target: { value: "grace@" } });
    expect(screen.getByText("Grace Hopper")).toBeInTheDocument();
    expect(screen.queryByText("Alan Turing")).toBeNull();

    fireEvent.change(search, { target: { value: "nobody" } });
    expect(screen.getByText("No members match this search.")).toBeInTheDocument();
  });

  it("shows when each pending invitation was sent and expires", async () => {
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("tab", { name: /Pending invitations\s*1/ }));
    expect(screen.getByText("linus@acme.dev")).toBeInTheDocument();
    expect(screen.getByText(/Sent .* · Expires /)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Revoke invitation for linus@acme.dev" }),
    ).toBeInTheDocument();
  });

  it("keeps the open list in the URL and follows it while on the page", async () => {
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("tab", { name: /Pending invitations\s*1/ }));
    expect(url.get()).toBe("tab=members&section=invitations");

    // A search result or back/forward changes the URL without remounting.
    act(() => url.set("tab=members&section=links"));
    expect(screen.getByRole("tab", { name: /Share links\s*0/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    act(() => url.set("tab=members"));
    expect(screen.getByRole("tab", { name: /Members\s*3/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("opens the tab a deep link names", () => {
    url.set("tab=members&section=links");
    renderWithI18n(<MembersTab />);
    expect(screen.getByRole("tab", { name: /Share links\s*0/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("No share links yet.")).toBeInTheDocument();
  });

  it("invites from a dialog and closes it once the invitation is sent", async () => {
    mockCreateMember.mockResolvedValue({});
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("button", { name: "Invite member" }));
    const dialog = await screen.findByRole("dialog", { name: "Invite member" });
    await user.type(within(dialog).getByRole("textbox", { name: "Email" }), "new@acme.dev");
    await user.click(within(dialog).getByRole("button", { name: "Invite" }));

    await waitFor(() =>
      expect(mockCreateMember).toHaveBeenCalledWith("ws-1", {
        email: "new@acme.dev",
        role: "member",
      }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("keeps the invite dialog open when sending fails", async () => {
    mockCreateMember.mockRejectedValue(new Error("nope"));
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("button", { name: "Invite member" }));
    const dialog = await screen.findByRole("dialog", { name: "Invite member" });
    await user.type(within(dialog).getByRole("textbox", { name: "Email" }), "new@acme.dev");
    await user.click(within(dialog).getByRole("button", { name: "Invite" }));

    await waitFor(() => expect(mockCreateMember).toHaveBeenCalled());
    expect(screen.getByRole("dialog", { name: "Invite member" })).toBeInTheDocument();
  });

  it("invites a client only once an organisation is chosen", async () => {
    mockCreateMember.mockResolvedValue({});
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("button", { name: "Invite member" }));
    const dialog = await screen.findByRole("dialog", { name: "Invite member" });
    await user.type(within(dialog).getByRole("textbox", { name: "Email" }), "nasir@client.dev");
    await user.click(within(dialog).getByRole("combobox", { name: "Role" }));
    await user.click(await screen.findByRole("option", { name: /^Client/ }));

    const invite = within(dialog).getByRole("button", { name: "Invite" });
    expect(invite).toBeDisabled();

    await user.click(within(dialog).getByRole("button", { name: "Organisations" }));
    await user.click(await screen.findByRole("button", { name: "Northwind" }));
    await user.keyboard("{Escape}");
    expect(within(dialog).getByText("Northwind")).toBeInTheDocument();
    expect(invite).toBeEnabled();

    await user.click(invite);
    await waitFor(() =>
      expect(mockCreateMember).toHaveBeenCalledWith("ws-1", {
        email: "nasir@client.dev",
        role: "client",
        organisation_ids: ["org-a"],
      }),
    );
  });

  it("shows a client's organisations and changes them from the row menu", async () => {
    mockUpdateMember.mockResolvedValue({});
    data.members = [
      member("user-1", "Ada Lovelace", "owner"),
      member("user-4", "Nasir Client", "client", ["org-a", "org-b"]),
    ];
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    expect(screen.getByText("Northwind")).toBeInTheDocument();
    expect(screen.getByText("Globex")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Actions for Nasir Client" }));
    await user.click(await screen.findByRole("menuitem", { name: "Organisations" }));
    // The opening submenu animates with pointer-events off, so user-event refuses the click.
    fireEvent.click(await screen.findByRole("menuitem", { name: "Globex" }));

    await waitFor(() =>
      expect(mockUpdateMember).toHaveBeenCalledWith("ws-1", "m-user-4", {
        role: "client",
        organisation_ids: ["org-a"],
      }),
    );
  });

  it("keeps a client's last organisation", async () => {
    data.members = [
      member("user-1", "Ada Lovelace", "owner"),
      member("user-4", "Nasir Client", "client", ["org-a"]),
    ];
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("button", { name: "Actions for Nasir Client" }));
    await user.click(await screen.findByRole("menuitem", { name: "Organisations" }));
    expect(await screen.findByRole("menuitem", { name: "Northwind" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("starts the next organisation edit from the refreshed set", async () => {
    mockUpdateMember.mockResolvedValue({});
    let finishRefresh = () => {};
    mockInvalidateQueries.mockImplementationOnce(
      () => new Promise<void>((resolve) => (finishRefresh = resolve)),
    );
    data.organisations.push({ id: "org-c", name: "Initech" });
    data.members = [
      member("user-1", "Ada Lovelace", "owner"),
      member("user-4", "Nasir Client", "client", ["org-a"]),
    ];
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);
    const actions = () => screen.getByRole("button", { name: "Actions for Nasir Client" });

    await user.click(actions());
    await user.click(await screen.findByRole("menuitem", { name: "Organisations" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Globex" }));
    await waitFor(() =>
      expect(mockUpdateMember).toHaveBeenCalledWith("ws-1", "m-user-4", {
        role: "client",
        organisation_ids: ["org-a", "org-b"],
      }),
    );
    await waitFor(() => expect(mockInvalidateQueries).toHaveBeenCalled());
    expect(actions()).toBeDisabled();

    data.members = [
      member("user-1", "Ada Lovelace", "owner"),
      member("user-4", "Nasir Client", "client", ["org-a", "org-b"]),
    ];
    await act(async () => finishRefresh());
    await waitFor(() => expect(actions()).toBeEnabled());

    await user.click(actions());
    await user.click(await screen.findByRole("menuitem", { name: "Organisations" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Initech" }));
    await waitFor(() =>
      expect(mockUpdateMember).toHaveBeenLastCalledWith("ws-1", "m-user-4", {
        role: "client",
        organisation_ids: ["org-a", "org-b", "org-c"],
      }),
    );
  });

  it("keeps a client's current role from sending an update", async () => {
    data.members = [
      member("user-1", "Ada Lovelace", "owner"),
      member("user-4", "Nasir Client", "client", ["org-a"]),
    ];
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("button", { name: "Actions for Nasir Client" }));
    await user.click(await screen.findByRole("menuitem", { name: "Change role" }));
    const current = await screen.findByRole("menuitem", { name: /^Client/ });
    expect(current).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(current);
    expect(mockUpdateMember).not.toHaveBeenCalled();
  });

  it("offers staff no client role", async () => {
    const user = userEvent.setup();
    renderWithI18n(<MembersTab />);

    await user.click(screen.getByRole("button", { name: "Actions for Alan Turing" }));
    await user.click(await screen.findByRole("menuitem", { name: "Change role" }));
    expect(await screen.findByRole("menuitem", { name: /^Admin/ })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /^Client/ })).toBeNull();
  });

  it("links seat usage to billing when subscriptions are on", () => {
    configStore
      .getState()
      .setFeatureFlags({ [BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG]: true });
    data.summary = { seatCapacity: { used: 3, reserved: 1, purchased: 5 } };
    renderWithI18n(<MembersTab />);

    expect(screen.getByText(/4 of 5 seats used/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Manage seats" })).toHaveAttribute(
      "href",
      "/acme/settings?tab=billing",
    );
  });

  it("gives members a read-only view without invite or share-link controls", () => {
    data.members = [
      member("user-1", "Ada Lovelace", "member"),
      member("user-2", "Grace Hopper", "owner"),
    ];
    renderWithI18n(<MembersTab />);

    expect(screen.getByRole("note")).toHaveTextContent("Ask Grace Hopper");
    expect(screen.queryByRole("button", { name: "Invite member" })).toBeNull();
    expect(screen.queryByRole("tab", { name: /Share links/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Actions for/ })).toBeNull();
  });
});
