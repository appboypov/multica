// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => vi.unstubAllGlobals());

const skill = {
  id: "skill-1",
  workspace_id: "ws-1",
  name: "review",
  description: "",
  config: {},
  created_by: null,
  created_at: "",
  updated_at: "",
  enabled: false,
  labels: [],
};

function respond(body: unknown, status = 200) {
  const fetch = vi.fn().mockResolvedValue(
    new Response(status === 204 ? null : JSON.stringify(body), { status }),
  );
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

describe("squad skills API", () => {
  it.each([
    ["list", (c: ApiClient) => c.listSquadSkills("squad-1"), "/api/squads/squad-1/skills", "GET", undefined],
    ["set", (c: ApiClient) => c.setSquadSkills("squad-1", { skill_ids: ["skill-1"] }), "/api/squads/squad-1/skills", "PUT", { skill_ids: ["skill-1"] }],
    ["add", (c: ApiClient) => c.addSquadSkills("squad-1", { skill_ids: ["skill-1"] }), "/api/squads/squad-1/skills/add", "POST", { skill_ids: ["skill-1"] }],
    ["enable", (c: ApiClient) => c.setSquadSkillEnabled("squad-1", "skill-1", false), "/api/squads/squad-1/skills/skill-1/enabled", "PUT", { enabled: false }],
    ["remove", (c: ApiClient) => c.removeSquadSkill("squad-1", "skill-1"), "/api/squads/squad-1/skills/skill-1", "DELETE", undefined],
  ] as const)("%s answers with the squad's parsed assigned skills", async (_name, call, path, method, body) => {
    const fetch = respond([skill]);
    await expect(call(new ApiClient("https://example.test"))).resolves.toEqual([skill]);
    expect(fetch).toHaveBeenCalledWith(
      `https://example.test${path}`,
      expect.objectContaining({ ...(method === "GET" ? {} : { method }), ...(body ? { body: JSON.stringify(body) } : {}) }),
    );
  });

  it("degrades a malformed assigned skill list to no skills", async () => {
    respond({ skills: "invalid" });
    await expect(new ApiClient("https://example.test").listSquadSkills("squad-1")).resolves.toEqual([]);
  });

  it("names the runtime when it sets one runtime skill", async () => {
    const fetch = respond(null, 204);
    const data = { runtime_id: "runtime-1", root: "provider" as const, key: "review", name: "Review", enabled: true };
    await new ApiClient("https://example.test").setSquadRuntimeSkillEnabled("squad-1", data);
    expect(fetch).toHaveBeenCalledWith(
      "https://example.test/api/squads/squad-1/runtime-skills/enabled",
      expect.objectContaining({ method: "PUT", body: JSON.stringify(data) }),
    );
  });

  it("sets the runtime skill section switch", async () => {
    const fetch = respond(null, 204);
    await new ApiClient("https://example.test").setSquadRuntimeSkillsInherited("squad-1", true);
    expect(fetch).toHaveBeenCalledWith(
      "https://example.test/api/squads/squad-1/runtime-skills/inherit",
      expect.objectContaining({ method: "PUT", body: JSON.stringify({ enabled: true }) }),
    );
  });
});
