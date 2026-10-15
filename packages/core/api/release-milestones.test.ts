// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import { EMPTY_RELEASE, ReleaseSchema } from "./schemas";

const milestone = { id: "m-1", release_id: "release-1", name: "UI prototype", date: "2026-11-02", created_at: "", updated_at: "" };
const release = { ...EMPTY_RELEASE, id: "release-1", repository_id: "repo-1", title: "First", version: "1.0.0", milestones: [milestone] };
function respond(body: unknown, status = 200) {
  const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}
afterEach(() => vi.unstubAllGlobals());

describe("release milestones API boundary", () => {
  it("preserves calendar days and defaults older or malformed milestone lists", () => {
    expect(ReleaseSchema.parse(release).milestones).toEqual([milestone]);
    for (const milestones of [undefined, null, "broken", {}]) {
      expect(ReleaseSchema.parse({ ...release, milestones }).milestones).toEqual([]);
    }
    expect(ReleaseSchema.parse({ ...release, milestones: [
      milestone, null, { ...milestone, date: "2026-02-30" },
      { ...milestone, name: " " }, { ...milestone, date: "2026-11-02T00:00:00Z" },
    ] }).milestones).toEqual([milestone]);
  });

  it("includes milestones in list, get and every release line response", async () => {
    const client = new ApiClient("https://api.example.test");
    respond({ releases: [release] });
    await expect(client.listReleases()).resolves.toEqual({ releases: [release] });
    respond(release);
    await expect(client.getRelease(release.id)).resolves.toEqual(release);
    respond({ releases: [release] });
    await expect(client.editRelease(release.id, { title: "First", version: "1.0.0" })).resolves.toEqual({ releases: [release] });
    await expect(client.planRelease("repo-1", { title: "First", bump: "minor" })).resolves.toEqual({ releases: [release] });
    await expect(client.moveRelease(release.id, { direction: "later" })).resolves.toEqual({ releases: [release] });
  });

  it("parses milestones on progress totals and defaults older totals", async () => {
    const client = new ApiClient("https://api.example.test");
    respond({ rows: [], releases: [{ release_id: release.id, done: 1, total: 2, milestones: [milestone] }] });
    expect((await client.listReleaseProgress()).releases[0]?.milestones).toEqual([milestone]);
    respond({ rows: [], releases: [{ release_id: release.id, done: 1, total: 2 }] });
    expect((await client.listReleaseProgress()).releases[0]?.milestones).toEqual([]);
  });

  it("sends milestone add, partial edit and remove with parsed line responses", async () => {
    const client = new ApiClient("https://api.example.test");
    const fetchMock = respond({ releases: [release] });
    const fields = { name: milestone.name, date: milestone.date };
    await expect(client.addReleaseMilestone(release.id, fields)).resolves.toEqual({ releases: [release] });
    await expect(client.editReleaseMilestone(milestone.id, { date: "2026-11-03" })).resolves.toEqual({ releases: [release] });
    await expect(client.removeReleaseMilestone(milestone.id)).resolves.toEqual({ releases: [release] });
    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://api.example.test/api/releases/release-1/milestones");
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ method: "POST", body: JSON.stringify(fields) });
    expect(fetchMock.mock.calls[1]?.[0]).toBe("https://api.example.test/api/release-milestones/m-1");
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({ method: "PATCH", body: JSON.stringify({ date: "2026-11-03" }) });
    expect(fetchMock.mock.calls[2]?.[1]).toMatchObject({ method: "DELETE" });
  });

  it("degrades malformed mutation responses and preserves HTTP failures", async () => {
    const client = new ApiClient("https://api.example.test");
    respond(null);
    const fields = { name: milestone.name, date: milestone.date };
    await expect(client.addReleaseMilestone(release.id, fields)).resolves.toEqual({ releases: [] });
    await expect(client.editReleaseMilestone(milestone.id, fields)).resolves.toEqual({ releases: [] });
    await expect(client.removeReleaseMilestone(milestone.id)).resolves.toEqual({ releases: [] });
    await expect(client.getRelease(release.id)).resolves.toEqual(EMPTY_RELEASE);
    respond({ error: "milestone not found" }, 404);
    await expect(client.editReleaseMilestone(milestone.id, fields)).rejects.toBeInstanceOf(ApiError);
    await expect(client.removeReleaseMilestone(milestone.id)).rejects.toBeInstanceOf(ApiError);
  });
});
