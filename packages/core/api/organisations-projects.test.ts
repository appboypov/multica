// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import { EMPTY_PROJECT, EMPTY_ORGANISATION } from "./schemas";

function respond(body: unknown, status = 200) {
  const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => vi.unstubAllGlobals());

const organisation = { id: "org-1", workspace_id: "ws-1", name: "Rebl Records", key: "rr" };
const project = { ...EMPTY_PROJECT, id: "project-1", workspace_id: "ws-1", title: "Rebl" };

describe("organisation API boundary", () => {
  it("parses the list, drops a row without a key, and defaults missing lists", async () => {
    const client = new ApiClient("https://api.example.test");
    respond({ organisations: [organisation, { id: "bad", workspace_id: "ws-1", name: "Bad" }], total: 2 });
    expect((await client.listOrganisations()).organisations).toEqual([
      { ...organisation, created_at: "", updated_at: "" },
    ]);
    respond({});
    await expect(client.listOrganisations()).resolves.toEqual({ organisations: [], total: 0 });
  });

  it("reads, creates and updates the key with parsed organisation responses", async () => {
    const client = new ApiClient("https://api.example.test");
    const fetchMock = respond(organisation);
    await expect(client.getOrganisation("org-1")).resolves.toMatchObject(organisation);
    await expect(client.createOrganisation({ name: "Rebl Records", key: "rr" })).resolves.toMatchObject(organisation);
    await expect(client.updateOrganisation("org-1", { key: "rebl" })).resolves.toMatchObject(organisation);
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({ method: "POST", body: JSON.stringify({ name: "Rebl Records", key: "rr" }) });
    expect(fetchMock.mock.calls[2]?.[1]).toMatchObject({ method: "PUT", body: JSON.stringify({ key: "rebl" }) });
    respond({ ...organisation, key: undefined });
    await expect(client.getOrganisation("org-1")).resolves.toEqual(EMPTY_ORGANISATION);
    await expect(client.createOrganisation({ name: "Rebl Records", key: "rr" })).resolves.toEqual(EMPTY_ORGANISATION);
    await expect(client.updateOrganisation("org-1", { key: "rebl" })).resolves.toEqual(EMPTY_ORGANISATION);
  });

  it("preserves server conflicts instead of treating them as parsed successes", async () => {
    respond({ error: "organisation key is taken" }, 409);
    const client = new ApiClient("https://api.example.test");
    await expect(client.updateOrganisation("org-1", { key: "das" })).rejects.toBeInstanceOf(ApiError);
  });
});

describe("project CRUD boundary", () => {
  it("parses old responses without organisation, slug or dates and defaults unknown enums", async () => {
    const old = { ...project, organisation_id: undefined, slug: undefined, start_date: undefined, due_date: undefined,
      status: "future-status", priority: "future-priority", lead_type: "future-lead" };
    const expected = { ...project, organisation_id: null, slug: null, start_date: null, due_date: null };
    const client = new ApiClient("https://api.example.test");
    respond({ projects: [old], total: 1 });
    await expect(client.listProjects()).resolves.toEqual({ projects: [expected], total: 1 });
    respond(old);
    await expect(client.getProject("project-1")).resolves.toEqual(expected);
    await expect(client.createProject({ title: "Rebl", organisation_id: "org-1" })).resolves.toEqual(expected);
    await expect(client.updateProject("project-1", { organisation_id: null })).resolves.toEqual(expected);
  });

  it("reads the slug, sends it on create and update, and reads a malformed slug as none", async () => {
    const client = new ApiClient("https://api.example.test");
    const fetchMock = respond({ ...project, organisation_id: "org-1", slug: "rebl-app" });
    await expect(client.getProject("project-1")).resolves.toMatchObject({ slug: "rebl-app" });
    await client.createProject({ title: "Rebl app", organisation_id: "org-1", slug: "rebl-app" });
    await client.updateProject("project-1", { slug: null });
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ title: "Rebl app", organisation_id: "org-1", slug: "rebl-app" }),
    });
    expect(fetchMock.mock.calls[2]?.[1]).toMatchObject({ method: "PUT", body: JSON.stringify({ slug: null }) });
    respond({ projects: [{ ...project, slug: 42 }], total: 1 });
    await expect(client.listProjects()).resolves.toEqual({ projects: [{ ...project, slug: null }], total: 1 });
  });

  it("preserves a taken slug as a server conflict", async () => {
    respond({ error: "project slug is taken", code: "project_slug_taken" }, 409);
    const client = new ApiClient("https://api.example.test");
    await expect(client.updateProject("project-1", { slug: "rebl" })).rejects.toBeInstanceOf(ApiError);
  });

  it("degrades malformed responses at every CRUD read boundary", async () => {
    respond(null);
    const client = new ApiClient("https://api.example.test");
    await expect(client.listProjects()).resolves.toEqual({ projects: [], total: 0 });
    await expect(client.getProject("project-1")).resolves.toEqual(EMPTY_PROJECT);
    await expect(client.createProject({ title: "Rebl" })).resolves.toEqual(EMPTY_PROJECT);
    await expect(client.updateProject("project-1", { organisation_id: "org-1" })).resolves.toEqual(EMPTY_PROJECT);
  });
});
