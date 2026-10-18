// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { MemberWithUser } from "../types";
import { clientsOfOrganisation, issueOrganisationId, toggleClientOrganisation } from "./clients";

const projects = [
  { id: "p-a", organisation_id: "org-a" },
  { id: "p-a2", organisation_id: "org-a" },
  { id: "p-b", organisation_id: "org-b" },
  { id: "p-none", organisation_id: null },
];

describe("issueOrganisationId", () => {
  it.each([
    { name: "one project of an organisation", ids: ["p-a"], want: "org-a" },
    { name: "two projects of one organisation", ids: ["p-a", "p-a2"], want: "org-a" },
    { name: "projects of two organisations", ids: ["p-a", "p-b"], want: null },
    { name: "an issue without a project", ids: [null], want: null },
    { name: "one issue without a project among others", ids: ["p-a", null], want: null },
    { name: "a project without an organisation", ids: ["p-none"], want: null },
    { name: "an unknown project", ids: ["p-gone"], want: null },
    { name: "no issues", ids: [], want: null },
  ])("gives $want for $name", ({ ids, want }) => {
    expect(issueOrganisationId(ids, projects)).toBe(want);
  });
});

function member(id: string, role: MemberWithUser["role"], organisation_ids: string[] = []): MemberWithUser {
  return {
    id, workspace_id: "ws", user_id: `user-${id}`, role, created_at: "", name: id, email: "",
    avatar_url: null, organisation_ids,
  };
}

describe("clientsOfOrganisation", () => {
  const members = [
    member("brian", "owner"),
    member("nasir", "client", ["org-a", "org-b"]),
    member("ralph", "client", ["org-b"]),
  ];

  it("gives only the clients of the organisation", () => {
    expect(clientsOfOrganisation(members, "org-a").map((m) => m.id)).toEqual(["nasir"]);
    expect(clientsOfOrganisation(members, "org-b").map((m) => m.id)).toEqual(["nasir", "ralph"]);
  });

  it("gives no clients without an organisation", () => {
    expect(clientsOfOrganisation(members, null)).toEqual([]);
  });
});

describe("toggleClientOrganisation", () => {
  it("adds an organisation the client lacks", () => {
    expect(toggleClientOrganisation(["org-a"], "org-b")).toEqual(["org-a", "org-b"]);
  });

  it("removes an organisation the client holds", () => {
    expect(toggleClientOrganisation(["org-a", "org-b"], "org-a")).toEqual(["org-b"]);
  });

  it("keeps the last organisation", () => {
    expect(toggleClientOrganisation(["org-a"], "org-a")).toEqual(["org-a"]);
  });
});
