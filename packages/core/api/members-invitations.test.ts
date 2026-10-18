// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import { EMPTY_INVITATION } from "./schemas";

function respond(body: unknown, status = 200) {
  const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => vi.unstubAllGlobals());

const member = {
  id: "membership-1",
  workspace_id: "ws-1",
  user_id: "user-1",
  role: "member",
  organisation_ids: [],
  created_at: "2026-10-15T10:00:00Z",
  name: "Sam",
  email: "sam@example.test",
  avatar_url: "https://example.test/sam.png",
};

const clientMember = {
  ...member,
  id: "membership-client",
  user_id: "user-client",
  role: "client",
  organisation_ids: ["org-1", "org-2"],
  name: "Rizgar",
  email: "rizgar@rebl.nl",
  avatar_url: null,
};

const invitation = {
  id: "invitation-1",
  workspace_id: "ws-1",
  inviter_id: "user-owner",
  invitee_email: "sam@example.test",
  invitee_user_id: "user-1",
  role: "member",
  organisation_ids: [],
  status: "pending",
  created_at: "2026-10-15T10:00:00Z",
  updated_at: "2026-10-15T10:01:00Z",
  expires_at: "2026-10-22T10:00:00Z",
  inviter_name: "Brian",
  inviter_email: "brian@example.test",
  workspace_name: "Rebl",
};

const clientInvitation = {
  ...invitation,
  id: "invitation-client",
  invitee_email: "rizgar@rebl.nl",
  invitee_user_id: null,
  role: "client",
  organisation_ids: ["org-1", "org-2"],
};

const malformedLists = [
  { name: "null", body: null },
  { name: "an object", body: {} },
  { name: "a string", body: "not a list" },
  { name: "a non-object row", body: [null] },
];

describe("member API boundary", () => {
  it.each([
    ["client", "client"],
    ["future-role", "member"],
  ])("keeps every member and their fields when a row has role %s", async (role, parsed) => {
    const staff = [
      { ...member, id: "membership-owner", user_id: "user-owner", role: "owner" },
      { ...member, id: "membership-admin", user_id: "user-admin", role: "admin" },
      member,
    ];
    respond([...staff, { ...clientMember, role }]);
    const client = new ApiClient("https://api.example.test");

    await expect(client.listMembers("ws-1")).resolves.toEqual([
      ...staff,
      { ...clientMember, role: parsed },
    ]);
  });

  it("defaults optional member fields without losing the membership or user identity", async () => {
    respond([{ id: "membership-client", workspace_id: "ws-1", user_id: "user-client", role: "client" }]);
    const client = new ApiClient("https://api.example.test");

    await expect(client.listMembers("ws-1")).resolves.toEqual([{
      id: "membership-client",
      workspace_id: "ws-1",
      user_id: "user-client",
      role: "client",
      created_at: "",
      name: "",
      email: "",
      avatar_url: null,
      organisation_ids: [],
    }]);
  });

  it("reads malformed organisations as none", async () => {
    respond([{ ...clientMember, organisation_ids: "org-1" }]);
    const client = new ApiClient("https://api.example.test");

    await expect(client.listMembers("ws-1")).resolves.toEqual([{ ...clientMember, organisation_ids: [] }]);
  });

  it.each(malformedLists)("falls back to an empty list for $name", async ({ body }) => {
    respond(body);
    const client = new ApiClient("https://api.example.test");

    await expect(client.listMembers("ws-1")).resolves.toEqual([]);
  });

  it("falls back to an empty list when a member row has malformed identity", async () => {
    respond([member, { ...clientMember, user_id: null }]);
    const client = new ApiClient("https://api.example.test");

    await expect(client.listMembers("ws-1")).resolves.toEqual([]);
  });
});

const invitationLists = [
  { name: "workspace invitations", read: (client: ApiClient) => client.listWorkspaceInvitations("ws-1") },
  { name: "own invitations", read: (client: ApiClient) => client.listMyInvitations() },
];

describe.each(invitationLists)("$name API boundary", ({ read }) => {
  it.each([
    ["client", "client"],
    ["future-role", "member"],
  ])("keeps every invitation and their fields when a row has role %s", async (role, parsed) => {
    const staff = [
      invitation,
      { ...invitation, id: "invitation-admin", role: "admin", status: "accepted" },
      { ...invitation, id: "invitation-declined", status: "declined" },
      { ...invitation, id: "invitation-expired", status: "expired" },
    ];
    respond([...staff, { ...clientInvitation, role }]);
    const client = new ApiClient("https://api.example.test");

    await expect(read(client)).resolves.toEqual([
      ...staff,
      { ...clientInvitation, role: parsed },
    ]);
  });

  it("defaults optional invitation fields without losing its identity or addressee", async () => {
    respond([{
      id: "invitation-client", workspace_id: "ws-1", inviter_id: "user-owner",
      invitee_email: "rizgar@rebl.nl", role: "client",
    }]);
    const client = new ApiClient("https://api.example.test");

    await expect(read(client)).resolves.toEqual([{
      ...EMPTY_INVITATION,
      id: "invitation-client",
      workspace_id: "ws-1",
      inviter_id: "user-owner",
      invitee_email: "rizgar@rebl.nl",
      role: "client",
    }]);
  });

  it.each(malformedLists)("falls back to an empty list for $name", async ({ body }) => {
    respond(body);
    const client = new ApiClient("https://api.example.test");

    await expect(read(client)).resolves.toEqual([]);
  });

  it("falls back to an empty list when an invitation row has malformed identity", async () => {
    respond([invitation, { ...clientInvitation, id: null }]);
    const client = new ApiClient("https://api.example.test");

    await expect(read(client)).resolves.toEqual([]);
  });
});

describe("single invitation API boundary", () => {
  it.each([
    ["client", "client"],
    ["future-role", "member"],
  ])("keeps the invitation and its fields when its role is %s", async (role, parsed) => {
    respond({ ...clientInvitation, role });
    const client = new ApiClient("https://api.example.test");

    await expect(client.getInvitation("invitation-client")).resolves.toEqual({
      ...clientInvitation, role: parsed,
    });
  });

  it("defaults optional fields on a single invitation", async () => {
    respond({
      id: "invitation-client", workspace_id: "ws-1", inviter_id: "user-owner",
      invitee_email: "rizgar@rebl.nl", role: "client",
    });
    const client = new ApiClient("https://api.example.test");

    await expect(client.getInvitation("invitation-client")).resolves.toEqual({
      ...EMPTY_INVITATION,
      id: "invitation-client",
      workspace_id: "ws-1",
      inviter_id: "user-owner",
      invitee_email: "rizgar@rebl.nl",
      role: "client",
    });
  });

  it.each([
    { name: "null", body: null },
    { name: "an object without identity", body: {} },
    { name: "an array", body: [] },
    { name: "a malformed identity", body: { ...clientInvitation, id: null } },
  ])("uses an empty invitation for $name", async ({ body }) => {
    respond(body);
    const client = new ApiClient("https://api.example.test");

    await expect(client.getInvitation("invitation-client")).resolves.toEqual(EMPTY_INVITATION);
  });
});

describe("member and invitation request failures", () => {
  it("does not replace server access refusals with successful empty responses", async () => {
    respond({ error: "forbidden" }, 403);
    const client = new ApiClient("https://api.example.test");

    await expect(client.listMembers("ws-1")).rejects.toBeInstanceOf(ApiError);
    await expect(client.listWorkspaceInvitations("ws-1")).rejects.toBeInstanceOf(ApiError);
    await expect(client.listMyInvitations()).rejects.toBeInstanceOf(ApiError);
    await expect(client.getInvitation("invitation-client")).rejects.toBeInstanceOf(ApiError);
  });
});
