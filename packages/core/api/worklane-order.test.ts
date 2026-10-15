// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { MoveWorklaneIssueResponseSchema } from "./schemas";

afterEach(() => vi.unstubAllGlobals());

describe("worklane move API", () => {
  it("posts one relative anchor and parses the canonical order", async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ issues: [] }), { status: 200 }));
    vi.stubGlobal("fetch", fetch);
    await expect(new ApiClient("https://example.test").moveWorklaneIssue("lane", "issue", { before: "other" })).resolves.toEqual([]);
    expect(fetch).toHaveBeenCalledWith("https://example.test/api/worklanes/lane/issues/issue/move", expect.objectContaining({ method: "POST", body: JSON.stringify({ before: "other" }) }));
  });

  it.each([null, {}, { issues: null }, { issues: "invalid" }, { issues: [{ id: "incomplete" }] }])("rejects malformed orders so the optimistic move rolls back: %j", async (body) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status: 200 })));
    await expect(new ApiClient("https://example.test").moveWorklaneIssue("lane", "issue", { after: "other" })).rejects.toThrow("malformed worklane issues");
    expect(MoveWorklaneIssueResponseSchema.safeParse(body).success).toBe(false);
  });
});
